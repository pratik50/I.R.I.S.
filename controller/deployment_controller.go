package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type IrisReconciler struct {
	client.Client
	Prometheus *PrometheusClient
	Loki       *LokiClient // ← nayi addition
	ArgoCD     *ArgoCDClient // ← nayi addition
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
		logger.Info("🚨 FAILURE DETECTED",
			"deployment", name,
			"namespace", namespace,
		)

		// Step 3: Prometheus se metrics laao
		metrics, err := r.Prometheus.FetchDeploymentMetrics(ctx, name, namespace)
		if err != nil {
			logger.Error(err, "Prometheus se metrics nahi mili")
		} else {
			logger.Info("📊 METRICS COLLECTED",
				"deployment", name,
				"available_replicas", metrics.AvailableReplicas,
				"cpu_cores", fmt.Sprintf("%.4f", metrics.CPUUsage),
				"memory_mb", fmt.Sprintf("%.2f MB", metrics.MemoryMB()),
			)
		}

		// Step 4: Loki se logs laao
		logs, err := r.Loki.FetchErrorLogs(ctx, name, namespace, 5*time.Minute)
		if err != nil {
			logger.Error(err, "Loki se logs nahi mile")
		} else if len(logs) == 0 {
			logger.Info("📋 NO ERROR LOGS FOUND",
				"deployment", name,
			)
		} else {
			logger.Info("📋 ERROR LOGS COLLECTED",
				"deployment", name,
				"total_logs", len(logs),
			)
			// Har log line print karo
			for i, logLine := range logs {
				logger.Info(fmt.Sprintf("  LOG %d: %s", i+1, logLine))
			}
		}

		// Step 5: Rollback trigger karo
		logger.Info("🔄 Triggering rollback via ArgoCD...",
			"deployment", name,
		)

		if err := r.ArgoCD.RollbackApp(ctx, "sample-app"); err != nil {
			logger.Error(err, "Rollback failed")
		} else {
			logger.Info("✅ ROLLBACK TRIGGERED SUCCESSFULLY",
				"deployment", name,
			)

			// Status check karo
			status, err := r.ArgoCD.GetAppStatus(ctx, "sample-app")
			if err == nil {
				logger.Info("📊 App status after rollback",
					"status", status,
				)
			}
		}

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