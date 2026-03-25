package controller

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// executeRollback performs the rollback via ArgoCD
func (r *IrisReconciler) executeRollback(ctx context.Context, deployment *appsv1.Deployment, reason string) error {
	logger := log.FromContext(ctx)
	name := deployment.Name
	namespace := deployment.Namespace

	if r.ArgoCD == nil {
		logger.Info("⏭️ Rollback skipped — ArgoCD client not configured")
		return nil
	}

	// Check cooldown
	if r.inRollbackCooldown(namespace, name) {
		logger.Info("⏳ Rollback cooldown active — skipping", "deployment", name)
		return nil
	}

	// Get app name from annotation (this is important for rollback)
	appName := deployment.Annotations["iris.argoproj.io/app"]
	if appName == "" {
		appName = name
		logger.Info("⚠️ No annotation found, using deployment name", "appName", appName)
	}

	logger.Info("🔄 Executing ArgoCD rollback...",
		"deployment", name,
		"app", appName,
		"reason", reason)

	if err := r.ArgoCD.RollbackApp(ctx, appName); err != nil {
		logger.Error(err, "❌ Rollback failed")
		return err
	}

	r.markRollback(namespace, name)
	logger.Info("✅ ROLLBACK SUCCESSFUL",
		"deployment", name,
		"app", appName)
	return nil
}

// inRollbackCooldown checks if rollback is allowed
func (r *IrisReconciler) inRollbackCooldown(namespace, name string) bool {
	if r.RollbackCooldown == 0 {
		return false
	}
	key := namespace + "/" + name
	r.mu.Lock()
	defer r.mu.Unlock()
	last, ok := r.LastRollback[key]
	if !ok {
		return false
	}
	return time.Since(last) < r.RollbackCooldown
}

// markRollback records the time of a rollback
func (r *IrisReconciler) markRollback(namespace, name string) {
	if r.RollbackCooldown == 0 {
		return
	}
	key := namespace + "/" + name
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.LastRollback == nil {
		r.LastRollback = make(map[string]time.Time)
	}
	r.LastRollback[key] = time.Now()
}