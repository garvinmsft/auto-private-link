package connection

import (
	"context"

	apl "github.com/garvinmsft/auto-private-link/pkg/apis/apl/v1alpha1"
	connClientset "github.com/garvinmsft/auto-private-link/pkg/generated/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	
)

const (
	connectionFinalizer =  "garvinmsft.github.com/apl-cleanup"
)

func needsCleanup(conn *apl.ServiceConnection) bool {
	return conn.DeletionTimestamp != nil
}

func hasFinalizer(conn *apl.ServiceConnection) bool {
	for _, finalizer := range conn.ObjectMeta.Finalizers {
		if finalizer == connectionFinalizer {
			return true
		}
	}
	return false
}

func (s *Controller) addFinalizer(client connClientset.Interface, conn *apl.ServiceConnection) error {
	if hasFinalizer(conn) {
		return nil
	}

	updated := conn.DeepCopy()
	updated.ObjectMeta.Finalizers = append(updated.ObjectMeta.Finalizers, connectionFinalizer)

	//klog.V(2).Infof("Adding finalizer to service %s/%s", updated.Namespace, updated.Name)
	
	err := updateConnection(client, updated)

	return err
}

func removeFinalizer(client connClientset.Interface, conn *apl.ServiceConnection) error {
	if !hasFinalizer(conn) {
		return nil
	}
	
	updated := conn.DeepCopy()
	var removed []string

	for _, item := range updated.ObjectMeta.Finalizers {
		if item != connectionFinalizer {
			removed = append(removed, item)
		}
	}

	updated.ObjectMeta.Finalizers = removed
	
	err := updateConnection(client, updated)

	return err
}

func updateConnection(client connClientset.Interface, conn *apl.ServiceConnection) error {

	ctx := context.TODO()

	_, err := 	client.AplV1alpha1().ServiceConnections(conn.Namespace).Update(ctx, conn,  metav1.UpdateOptions{})

	return err
}