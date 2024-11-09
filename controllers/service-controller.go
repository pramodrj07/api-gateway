package controllers

import (
	"context"
	"fmt"
	"net"
	"sync"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceReconciler is the controller that watches for Service events.
type ServiceReconciler struct {
	client.Client
	ServiceMap map[string]string // Map of service names to addresses
	Mutex      sync.RWMutex      // Protects concurrent access to ServiceMap
}

// Reconcile is called whenever a Service is created, updated, or deleted.
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the Service instance
	var svc corev1.Service
	if err := r.Get(ctx, req.NamespacedName, &svc); err != nil {
		log.Info("Service not found, might be deleted", "name", req.Name, "namespace", req.Namespace)
		r.Mutex.Lock()
		delete(r.ServiceMap, req.Name) // Remove from map if deleted
		r.Mutex.Unlock()
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Get the ClusterIP and port of the service
	var address string
	if len(svc.Spec.Ports) > 0 && svc.Spec.ClusterIP != corev1.ClusterIPNone {
		address = net.JoinHostPort(svc.Spec.ClusterIP, fmt.Sprint(svc.Spec.Ports[0].Port))
	}

	// Update the map with the service address
	r.Mutex.Lock()
	r.ServiceMap[svc.Name] = address
	r.Mutex.Unlock()

	log.Info("Updated service in map", "name", svc.Name, "address", address)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Complete(r)
}
