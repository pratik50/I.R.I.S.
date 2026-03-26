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

	// Ignore non-default namespace
	if namespace != "default" {
        logger.V(1).Info("Ignore non-default namespace", 
            "deployment", name, 
            "namespace", namespace)
        return ctrl.Result{}, nil
    }

	// Check if deployment is healthy
	if !r.detectFailure(deployment) {
		logger.Info("✅ Deployment healthy", "deployment", name, "available", deployment.Status.AvailableReplicas)
		return ctrl.Result{}, nil
	}

	logger.Info("🚨 FAILURE DETECTED", "deployment", name, "namespace", namespace)
	
	// Check if rollback happened recently (within cooldown)
	if r.inRollbackCooldown(namespace, name) {
		logger.Info("⏭️ Skipping notifications - rollback recently performed", 
			"deployment", name)
		return ctrl.Result{}, nil
	}

	restartCount := r.getRestartCount(ctx, deployment)

	if r.Slack != nil {
		if err := r.Slack.SendCrashDetected(name, namespace, restartCount); err != nil {
			logger.Error(err, "Failed to send crash detected alert")
		}
	}

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

		// Send Slack alert for CrashLoopBackOff
		if r.Slack != nil {
			if err := r.Slack.SendCrashLoopBackOffAlert(name, namespace, restartCount); err != nil {
				logger.Error(err, "Failed to send Slack notification for CrashLoopBackOff")
			}
		}

		if err := r.executeRollback(ctx, deployment, "CrashLoopBackOff detected"); err != nil {
			logger.Error(err, "Rollback failed")
			
			// Send Slack alert for rollback failure
			if r.Slack != nil {
				if err := r.Slack.SendRollbackFailure(name, namespace, err.Error()); err != nil {
					logger.Error(err, "Failed to send Slack notification for rollback failure")
				}
			}
		}

		return ctrl.Result{}, nil
	}

	// First crash or non-crashloop failure then proceed with AI analysis
	logger.Info("🤖 First crash detected - proceeding with AI analysis", "deployment", name)

	// Send AI analyzing alert
	if r.Slack != nil {
		if err := r.Slack.SendAIAnalyzing(name, namespace); err != nil {
			logger.Error(err, "Failed to send AI analyzing alert")
		}
	}

	// Collect data
	metrics, _ := r.collectMetrics(ctx, name, namespace)
	logs, _ := r.collectLogs(ctx, name, namespace)
	events, _ := r.fetchDeploymentEvents(ctx, name, namespace)

	// AI analysis
	analysis, _ := r.analyzeWithAI(ctx, name, namespace, metrics, logs, events)

	if analysis != nil && r.Slack != nil {
		if err := r.Slack.SendAIDecision(name, namespace, analysis.RootCause, analysis.Suggestion, analysis.RiskScore, analysis.Action); err != nil {
			logger.Error(err, "Failed to send AI decision alert")
		}
	}

	// Decision based on risk score
	if analysis != nil && analysis.RiskScore >= 0.5 {
		logger.Info("🔴 AUTO-ROLLBACK TRIGGERED",
			"deployment", name,
			"risk_score", analysis.RiskScore,
			"reason", analysis.RootCause)
		if err := r.executeRollback(ctx, deployment, analysis.RootCause); err != nil {
			logger.Error(err, "Rollback failed")
			
			// Send Slack alert for rollback failure
			if r.Slack != nil {
				if err := r.Slack.SendRollbackFailure(name, namespace, err.Error()); err != nil {
					logger.Error(err, "Failed to send Slack notification for rollback failure")
				}
			}
		}
	} else if analysis != nil {
		logger.Info("⚠️ MANUAL DIAGNOSIS REQUIRED",
			"deployment", name,
			"risk_score", analysis.RiskScore,
			"reason", analysis.RootCause,
			"suggestion", analysis.Suggestion)

		// Send Slack alert for manual diagnosis
		if r.Slack != nil {
			if err := r.Slack.SendManualDiagnosisRequired(name, namespace, analysis.RootCause, analysis.Suggestion, analysis.RiskScore); err != nil {
				logger.Error(err, "Failed to send Slack notification for manual diagnosis")
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller with the manager
func (r *IrisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Complete(r)
}