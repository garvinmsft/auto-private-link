package service


import (

	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"github.com/garvinmsft/auto-private-link/pkg/config"
	"github.com/garvinmsft/auto-private-link/pkg/azure"

)

const (
	component = "auto-private-link-service"
)



// Controller for private link service
type Controller struct {
	cfg                 config.Config
	azContext           azure.AzContext
	kubeClient          clientset.Interface
	serviceLister       corelisters.ServiceLister
	serviceListerSynced cache.InformerSynced
	queue workqueue.RateLimitingInterface
}

// New returns a new service controller to sync private link services in sync with k8s 
func New(
	kubeClient clientset.Interface,
	svcIformer coreinformers.ServiceInformer,
	cfg config.Config,
	azCtx azure.AzContext,

) (*Controller) {
	limiter := workqueue.NewItemExponentialFailureRateLimiter(cfg.MinRetryDelay, cfg.MaxRetryDelay)

	s := &Controller{
		kubeClient: kubeClient,
		cfg: cfg,
		azContext: azCtx,
		serviceListerSynced: svcIformer.Informer().HasSynced,
		serviceLister: svcIformer.Lister(),
		queue:  workqueue.NewNamedRateLimitingQueue(limiter, component),
	}

	svcIformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(cur interface{}) {
				svc, ok := cur.(*v1.Service)
				
				if ok && shouldProcess(svc, cfg.ServiceAnnotation) {
					s.enqueueService(svc)
				}
			},
			UpdateFunc: func(old, cur interface{}) {
				svcOld, okOld := old.(*v1.Service)
				svcCur, okCur := cur.(*v1.Service)
				if okOld && okCur && (shouldProcess(svcOld, cfg.ServiceAnnotation) || shouldProcess(svcCur, cfg.ServiceAnnotation)){ 
					s.enqueueService(svcCur)
				}
			},
		},
		cfg.SyncPeriod,
	)

	return s
}

//Run starts the controller
func (s *Controller) Run(stopCh <-chan struct{}, workers int) {

	klog.Info("Starting service controller")

	if !cache.WaitForNamedCacheSync(component, stopCh, s.serviceListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(s.serviceWorker, time.Second, stopCh)
	}

}

//ShutDown does cleanup when controller is terminated 
func (s *Controller) ShutDown() {
	klog.Info("Shutting down service controller")
	s.queue.ShutDown()
}

func (s *Controller) serviceWorker() {
	for s.processNextServiceItem() {
	}
}

func (s *Controller) enqueueService(service *v1.Service ) {

	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(service); err != nil {
		klog.Error(err.Error())
		return
	}
	s.queue.Add(key)
}

func (s *Controller) processNextServiceItem() bool {
	key, quit := s.queue.Get()
	if quit {
		return false
	}
	defer s.queue.Done(key)

	err := s.syncService(key.(string))
	if err == nil {
		s.queue.Forget(key)
		return true
	}

	klog.V(5).Infof("error processing service %v (will retry): %v", key, err)
	s.queue.AddRateLimited(key)
	return true
}

func (s *Controller) syncService(key string) error {

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.V(5).Infof("invalid resource key: %s", key)
		return nil
	}

	service, err := s.serviceLister.Services(namespace).Get(name)
	if err != nil {
		// The Foo resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			klog.V(5).Infof("Service '%s' in work queue no longer exists", key)
			return s.cleanupService(service)
		}

		if !shouldProcess(service, s.cfg.ServiceAnnotation) {
			klog.V(5).Infof("Service '%s' in work queue but can't be processed. Cleaning up", key)
			return s.cleanupService(service)
		}

		return err
	}

	if service.DeletionTimestamp != nil {
		return s.cleanupService(service)
	}

	 
	ok := isAPLService(service, s.cfg.ServiceAnnotation)

	//Should never happen ;)
	if !ok {
		return fmt.Errorf("Is not an APL service")
	}

	klog.V(5).Infof("Syncing for apl service: %v", service.Name)
	
	//Check finalizers
	err = s.addFinalizer(s.kubeClient, service)
	if err!= nil {
		return err
	}
	
	return s.azContext.AddUpdatePrivateService(service)

}


func (s *Controller) cleanupService(service *v1.Service ) error {

	err := s.azContext.RemoveService(service)

	if err != nil {
		return err
	}
	
	return removeFinalizer(s.kubeClient, service)
}


