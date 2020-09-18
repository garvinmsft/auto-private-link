package main

import (
	//"context"
	"os"
	"time"
	"k8s.io/klog"
	"github.com/spf13/pflag"
	kubeinformers "k8s.io/client-go/informers"

	"k8s.io/client-go/kubernetes" 

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/sample-controller/pkg/signals"
	clientset "github.com/garvinmsft/auto-private-link/pkg/generated/clientset/versioned"
	informers "github.com/garvinmsft/auto-private-link/pkg/generated/informers/externalversions"
	"github.com/garvinmsft/auto-private-link/pkg/config"
	"github.com/garvinmsft/auto-private-link/pkg/azure"
	"github.com/garvinmsft/auto-private-link/pkg/k8scontext"
	"github.com/garvinmsft/auto-private-link/pkg/controller/connection"
	"github.com/garvinmsft/auto-private-link/pkg/controller/service"

)



const (
	verbosityFlag = "verbosity"
	component = "auto-private-link"
)

var (
	flags          = pflag.NewFlagSet("auto-private-link", pflag.ExitOnError)
	kubeConfigFile = flags.String("kubeconfig", "", "Path to kubeconfig file with authorization and master location information.")
	versionInfo    = flags.Bool("version", false, "Print version")
	verbosity      = flags.Int(verbosityFlag, 5, "Set logging verbosity level")
)

func main() {
	defer klog.Flush()

	if err := flags.Parse(os.Args); err != nil {
		klog.Fatal("Error parsing command line arguments:", err)
	}
	//ctx := context.TODO()

	kubeCfg, err:= clientcmd.BuildConfigFromFlags("", *kubeConfigFile)

	if err!= nil {
		klog.Fatal("Error loading kubernetes config:", err)
	}

	kubeClient := kubernetes.NewForConfigOrDie(kubeCfg)
	aplClient := clientset.NewForConfigOrDie(kubeCfg)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	aplInformerFactory := informers.NewSharedInformerFactory(aplClient, time.Second*30)

	serviceInformer := kubeInformerFactory.Core().V1().Services()
	aplInformer := aplInformerFactory.Apl().V1alpha1().ServiceConnections()

	cfg, err := config.NewConfigFromEnv();
	if err != nil {
		klog.Fatal("Error parsing configuration values:", err)
	}

	recorder := k8scontext.NewEventRecorder(kubeClient, component)
	azCtx, err := azure.NewAzContext(cfg, recorder)
	if err != nil {
		klog.Fatal("Error parsing configuration values:", err)
	}

	stopCh := signals.SetupSignalHandler()
	
	//maybe build 2 separate binaries?
	svcController:= service.New(kubeClient, serviceInformer, cfg, azCtx)
	connController := connection.New(aplClient, kubeClient, aplInformer, serviceInformer, cfg, azCtx, recorder)

	kubeInformerFactory.Start(stopCh)
	aplInformerFactory.Start(stopCh)
	
	svcController.Run(stopCh, 1)
	connController.Run(stopCh, 1)

	klog.Info("Starting Services and connection controllers")
	<-stopCh

	klog.Info("Stopped controllers - Hope this is not a surprise")
}
