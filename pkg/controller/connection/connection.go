package connection

import (

	"fmt"
	"time"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	v1 "k8s.io/api/core/v1"
	apl "github.com/garvinmsft/auto-private-link/pkg/apis/apl/v1alpha1"
	"github.com/garvinmsft/auto-private-link/pkg/azure"
	"github.com/garvinmsft/auto-private-link/pkg/config"
	connClientset "github.com/garvinmsft/auto-private-link/pkg/generated/clientset/versioned"
	informers "github.com/garvinmsft/auto-private-link/pkg/generated/informers/externalversions/apl/v1alpha1"
	listers "github.com/garvinmsft/auto-private-link/pkg/generated/listers/apl/v1alpha1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"
)

const (
	component = "auto-private-link"
	controllerTag = "apl-connection"
	noServiceForPrivateConnection ="NoServiceForPrivateConnection"
)

var (
	
	policyDisabled = "Disabled"

)

// Controller keeps private link
type Controller struct {
	cfg                 config.Config
	azContext           azure.AzContext
	connClient          connClientset.Interface
	kubeClient          clientset.Interface
	connListerSynced    cache.InformerSynced
	connLister          listers.ServiceConnectionLister
	serviceLister       corelisters.ServiceLister
	serviceListerSynced cache.InformerSynced
	eventRecorder       record.EventRecorder
	queue workqueue.RateLimitingInterface
}

// New returns a new connection controller to keep sync private link connections
func New(
	connClient connClientset.Interface,
	kubeClient clientset.Interface,
	connInformer informers.ServiceConnectionInformer,
	svcIformer coreinformers.ServiceInformer,
	cfg config.Config,
	azCtx azure.AzContext,
	recorder record.EventRecorder,

) (*Controller) {

	limiter := workqueue.NewItemExponentialFailureRateLimiter(cfg.MinRetryDelay, cfg.MaxRetryDelay)

	s := &Controller{
		connClient:       connClient,
		cfg: cfg,
		azContext: azCtx,
		eventRecorder:    recorder,
		serviceListerSynced: svcIformer.Informer().HasSynced,
		serviceLister: svcIformer.Lister(),
		connLister: 	  connInformer.Lister(),
		connListerSynced: connInformer.Informer().HasSynced,
		queue:            workqueue.NewNamedRateLimitingQueue(limiter, controllerTag),
		
	}

	connInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(cur interface{}) {
				if conn, ok := cur.(*apl.ServiceConnection); ok{
					s.enqueueConnection(conn)
				}
				
			},
			UpdateFunc: func(old, cur interface{}) {
				if conn, ok := cur.(*apl.ServiceConnection); ok{
					s.enqueueConnection(conn)
				}
			},
		},
		cfg.SyncPeriod,
	)

	return s
}

//Run starts the controller
func (s *Controller) Run(stopCh <-chan struct{}, workers int) {

	klog.Info("Starting connection controller")

	if !cache.WaitForNamedCacheSync(controllerTag, stopCh, s.connListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(s.connWorker, time.Second, stopCh)
	}
}

//ShutDown does cleanup when controller is terminated 
func (s *Controller) ShutDown() {
	klog.Info("Shutting down connection controller")
	s.queue.ShutDown()
}

func (s *Controller) enqueueConnection(conn *apl.ServiceConnection ) {

	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(conn); err != nil {
		klog.Error(err.Error())
		return
	}
	s.queue.Add(key)
}

func (s *Controller) connWorker() {
	for s.processNextConnItem() {
	}
}

func (s *Controller) processNextConnItem() bool {
	key, quit := s.queue.Get()
	if quit {
		return false
	}
	defer s.queue.Done(key)

	err := s.syncConnection(key.(string))
	if err == nil {
		s.queue.Forget(key)
		return true
	}

	klog.V(5).Infof("error connection %v (will retry): %v", key, err)
	s.queue.AddRateLimited(key)
	return true
}




func (s *Controller) syncConnection(key string) error {

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.V(5).Infof("invalid resource key: %s", key)
		return nil
	}


	conn, err := s.connLister.ServiceConnections(namespace).Get(name)
	if err != nil {
		// The Foo resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			klog.V(5).Infof("connection '%s' in work queue no longer exists", key)
			return nil
		} 
		
		return err	
	}

	service, err := s.serviceLister.Services(namespace).Get(conn.Spec.ServiceName)

	if err!= nil {
		if errors.IsNotFound(err) {
			msg := fmt.Sprintf("Tried to sync connection: %s but service: %s does not exist in namespace: %s",  conn.Name, conn.Spec.ServiceName, namespace)
			klog.Warning(msg)
			s.eventRecorder.Event(conn, v1.EventTypeWarning, noServiceForPrivateConnection ,msg)
			return s.cleanupConnection(conn)
		}
		return err
	}

	if service.DeletionTimestamp != nil || conn.DeletionTimestamp != nil {
		return s.cleanupConnection(conn)
	}
	

	klog.V(5).Infof("Syncing for apl service connection: %v", conn.Name)

	err = s.addFinalizer(s.connClient, conn)
	if err!= nil {
		return err
	}
	
	return s.azContext.AddUpdatePrivateConnection(conn, conn.Spec.ServiceName)

}



func (s *Controller) cleanupConnection(conn *apl.ServiceConnection) error {
	
	err := s.azContext.RemoveEndpoint(conn)

	if err!= nil{
		return err
	}

	return removeFinalizer(s.connClient, conn)

}

