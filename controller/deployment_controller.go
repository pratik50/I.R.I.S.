package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// IrisReconciler — ab Prometheus client bhi hai andar
type IrisReconciler struct {
	client.Client
	Prometheus *PrometheusClient // ← nayi addition
}

func (r *IrisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 1: K8s se deployment info laao
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, deployment); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	name      := deployment.Name
	namespace := deployment.Namespace
	desired   := *deployment.Spec.Replicas
	available := deployment.Status.AvailableReplicas

	// Step 2: Failure check karo
	if available == 0 && desired > 0 {
		logger.Info("🚨 FAILURE DETECTED — fetching metrics...",
			"deployment", name,
			"namespace", namespace,
		)

		// Step 3: Prometheus se metrics laao
		metrics, err := r.Prometheus.FetchDeploymentMetrics(ctx, name, namespace)
		if err != nil {
			logger.Error(err, "Prometheus se metrics nahi mili")
			// Error aaya toh bhi continue karo — metrics optional hain abhi
		} else {
			logger.Info("📊 METRICS COLLECTED",
				"deployment", name,
				"available_replicas", metrics.AvailableReplicas,
				"cpu_cores", fmt.Sprintf("%.4f", metrics.CPUUsage),
				"memory_mb", fmt.Sprintf("%.2f MB", metrics.MemoryMB()),
			)
		}

		// Baad mein yahan aayega:
		// → logs fetch (Day 3)
		// → AI analysis (Day 7)
		// → rollback (Day 4)

	} else {
		logger.Info("✅ Deployment healthy",
			"deployment", name,
			"available", available,
		)
	}

	return ctrl.Result{}, nil
}

func (r *IrisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Complete(r)
}