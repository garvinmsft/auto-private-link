package service

import (
	"context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"

)

const (
	internalLoadBalancerKey = "service.beta.kubernetes.io/azure-load-balancer-internal"
    serviceFinalizer = "garvinmsft.github.com/apl-cleanup"
)

func shouldProcess(service *v1.Service, annotation string) bool {
	
	isILB := isILBService(service)
	hasIP := serviceHasIP(service)
	isAPL := isAPLService(service, annotation)
	
	return  isILB && hasIP && isAPL
}

func serviceHasIP(service *v1.Service) bool {
	return len(service.Status.LoadBalancer.Ingress) > 0
}



func isILBService(service *v1.Service) bool {
	if val, ok := service.Annotations[internalLoadBalancerKey]; ok {
		return val =="true" && service.Spec.Type == v1.ServiceTypeLoadBalancer
	}
	return false
}

func isAPLService(service *v1.Service, annotation string) bool {
	if val, ok := service.Annotations[annotation]; ok && val!="" {
		return val == "true"
	}
	return false
}

func hasFinalizer(service *v1.Service) bool {
	for _, finalizer := range service.ObjectMeta.Finalizers {
		if finalizer == serviceFinalizer {
			return true
		}
	}
	return false
}

func removeFinalizer(client clientset.Interface, service *v1.Service) error {
	if !hasFinalizer(service) {
		return nil
	}

	updated := service.DeepCopy()
	var removed []string

	for _, item := range updated.ObjectMeta.Finalizers {
		if item != serviceFinalizer {
			removed = append(removed, item)
		}
	}

	updated.ObjectMeta.Finalizers = removed
	
	err := updateService(client, updated)

	return err
}

func (s *Controller) addFinalizer(client clientset.Interface, service *v1.Service) error {
	if hasFinalizer(service) {
		return nil
	}
	updated := service.DeepCopy()
	updated.ObjectMeta.Finalizers = append(updated.ObjectMeta.Finalizers, serviceFinalizer)

	//klog.V(2).Infof("Adding finalizer to service %s/%s", updated.Namespace, updated.Name)
	
	err := updateService(client, updated)

	return err
}

func updateService(client clientset.Interface, service *v1.Service) error {

	_, err := client.CoreV1().Services(service.Namespace).Update(context.TODO(), service, metav1.UpdateOptions{})

	return err
}