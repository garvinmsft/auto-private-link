package k8scontext

import (
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
)

//NewEventRecorder creates the event recorder to be used by the controller
func NewEventRecorder(kubeClient clientset.Interface, component string) record.EventRecorder {
	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(klog.Infof)
	broadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: component})

	return recorder
}