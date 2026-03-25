package controller

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconcile is the main reconciliation loop
func (r *IrisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch deployment
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, deployment); err != nil {
		return ctrl.Result{}, ctrlclient.IgnoreNotFound(err)
	}

	name := deployment.Name
	namespace := deployment.Namespace

	// Check if deployment is healthy
	if !r.detectFailure(deployment) {
		logger.Info("✅ Deployment healthy", "deployment", name, "available", deployment.Status.AvailableReplicas)
		return ctrl.Result{}, nil
	}

	logger.Info("🚨 FAILURE DETECTED", "deployment", name, "namespace", namespace)

	// Determine desired replicas
	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}

	// If rollout is still in progress (no failure yet) then wait
	if r.isRolloutInProgress(deployment, desired) {
		logger.Info("⏳ Rollout in progress — waiting", "deployment", name)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Check for CrashLoopBackOff
	if forceRollback, restartCount := r.checkCrashLoopBackOff(ctx, deployment); forceRollback {
		logger.Info("🔴 FORCE ROLLBACK: CrashLoopBackOff detected",
			"deployment", name, "restart_count", restartCount)
		if err := r.executeRollback(ctx, deployment, "CrashLoopBackOff detected"); err != nil {
			logger.Error(err, "Rollback failed")
		}
		return ctrl.Result{}, nil
	}

	// First crash or non-crashloop failure then proceed with AI analysis
	logger.Info("🤖 First crash detected - proceeding with AI analysis", "deployment", name)

	// Collect data
	metrics, _ := r.collectMetrics(ctx, name, namespace)
	logs, _ := r.collectLogs(ctx, name, namespace)
	events, _ := r.fetchDeploymentEvents(ctx, name, namespace)

	// AI analysis
	analysis, _ := r.analyzeWithAI(ctx, name, namespace, metrics, logs, events)

	// Decision based on risk score
	if analysis != nil && analysis.RiskScore >= 0.5 {
		logger.Info("🔴 AUTO-ROLLBACK TRIGGERED",
			"deployment", name,
			"risk_score", analysis.RiskScore,
			"reason", analysis.RootCause)
		if err := r.executeRollback(ctx, deployment, analysis.RootCause); err != nil {
			logger.Error(err, "Rollback failed")
		}
	} else if analysis != nil {
		logger.Info("⚠️ MANUAL DIAGNOSIS REQUIRED",
			"deployment", name,
			"risk_score", analysis.RiskScore,
			"reason", analysis.RootCause,
			"suggestion", analysis.Suggestion)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller with the manager
func (r *IrisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Complete(r)
}