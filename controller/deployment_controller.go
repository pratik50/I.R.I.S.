package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// IrisReconciler — IRIS ka main struct
// client.Client = K8s se baat karne ka tool
type IrisReconciler struct {
	client.Client
}

// Reconcile — ye function tab call hoga jab bhi
// K8s mein koi Deployment change hoga
// (create, update, delete, crash — sab pe)
func (r *IrisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 1: K8s se deployment ki info laao
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, deployment); err != nil {
		// Deployment delete ho gayi — ignore karo
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Step 2: Failure check karo
	desired := *deployment.Spec.Replicas
	available := deployment.Status.AvailableReplicas

	if available == 0 && desired > 0 {
		logger.Info("🚨 FAILURE DETECTED",
			"deployment", deployment.Name,
			"namespace", deployment.Namespace,
			"desired_replicas", desired,
			"available_replicas", available,
		)
		// Baad mein yahan aayega:
		// → metrics fetch
		// → logs fetch
		// → AI analysis
		// → rollback
	} else {
		logger.Info("✅ Deployment healthy",
			"deployment", deployment.Name,
			"available", available,
		)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager — controller ko register karo
// "In deployments watch karo"
func (r *IrisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Complete(r)
}
