package controller

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type IrisReconciler struct {
	client.Client
	Prometheus *PrometheusClient
	Loki       *LokiClient 
	ArgoCD     *ArgoCDClient
}

func (r *IrisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 1: K8s se deployment info laao
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, deployment); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	name := deployment.Name
	namespace := deployment.Namespace
	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}
	available := deployment.Status.AvailableReplicas
	progressingCond := getDeploymentCondition(deployment.Status.Conditions, appsv1.DeploymentProgressing)
	availableCond := getDeploymentCondition(deployment.Status.Conditions, appsv1.DeploymentAvailable)

	// Failure = desired > 0 and not enough available, or condition says not progressing/available.
	failure := (desired > 0 && available < desired)
	if progressingCond != nil && progressingCond.Status == corev1.ConditionFalse {
		failure = true
	}
	if availableCond != nil && availableCond.Status == corev1.ConditionFalse {
		failure = true
	}

	// Step 2: Failure check karo
	if failure {
		logger.Info("🚨 FAILURE DETECTED",
			"deployment", name,
			"namespace", namespace,
			"desired", desired,
			"available", available,
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

		if len(logs) == 0 {
			logger.Info("📋 NO ERROR LOGS FOUND, fetching recent logs",
				"deployment", name,
			)
			recentLogs, err := r.Loki.FetchRecentLogs(ctx, name, namespace, 5*time.Minute)
			if err != nil {
				logger.Error(err, "Loki se recent logs nahi mile")
			} else if len(recentLogs) == 0 {
				logger.Info("📋 NO RECENT LOGS FOUND",
					"deployment", name,
				)
			} else {
				logger.Info("📋 RECENT LOGS COLLECTED",
					"deployment", name,
					"total_logs", len(recentLogs),
				)
				for i, logLine := range recentLogs {
					logger.Info(fmt.Sprintf("  LOG %d: %s", i+1, logLine))
				}
			}
		}

		events, err := r.fetchDeploymentEvents(ctx, name, namespace)
		if err != nil {
			logger.Error(err, "K8s events fetch nahi ho paaye")
		} else if len(events) == 0 {
			logger.Info("📌 NO K8S EVENTS FOUND",
				"deployment", name,
			)
		} else {
			logger.Info("📌 K8S EVENTS COLLECTED",
				"deployment", name,
				"total_events", len(events),
			)
			for i, eventLine := range events {
				logger.Info(fmt.Sprintf("  EVENT %d: %s", i+1, eventLine))
			}
		}

		// Step 5: ArgoCD rollback (optional)
		if r.ArgoCD == nil {
			logger.Info("⏭️ Rollback skipped — ArgoCD client not configured",
				"deployment", name,
				"namespace", namespace,
			)
		} else if namespace == "default" {
			if deployment.Annotations == nil || deployment.Annotations["iris.argoproj.io/app"] == "" {
				logger.Info("⏭️ Rollback skipped — ArgoCD app annotation missing",
					"deployment", name,
					"namespace", namespace,
				)
				return ctrl.Result{}, nil
			}

			appName := deployment.Annotations["iris.argoproj.io/app"]

			logger.Info("🔄 Triggering rollback via ArgoCD...",
				"deployment", name,
				"app", appName,
			)

			if err := r.ArgoCD.RollbackApp(ctx, appName); err != nil {
				logger.Error(err, "Rollback failed")
			} else {
				logger.Info("✅ ROLLBACK TRIGGERED SUCCESSFULLY",
					"deployment", name,
					"app", appName,
				)

				status, err := r.ArgoCD.GetAppStatus(ctx, appName)
				if err == nil {
					logger.Info("📊 App status after rollback",
						"status", status,
					)
				}
			}
		} else {
			logger.Info("⏭️ Rollback skipped — not default namespace",
				"deployment", name,
				"namespace", namespace,
			)
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

func getDeploymentCondition(conditions []appsv1.DeploymentCondition, condType appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

func (r *IrisReconciler) fetchDeploymentEvents(ctx context.Context, deploymentName, namespace string) ([]string, error) {
	var events corev1.EventList
	if err := r.List(ctx, &events, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	var filtered []corev1.Event
	for _, event := range events.Items {
		if event.InvolvedObject.Namespace != namespace {
			continue
		}
		if event.InvolvedObject.Kind == "Deployment" && event.InvolvedObject.Name == deploymentName {
			filtered = append(filtered, event)
			continue
		}
		if event.InvolvedObject.Kind == "Pod" && strings.HasPrefix(event.InvolvedObject.Name, deploymentName+"-") {
			filtered = append(filtered, event)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].LastTimestamp.Time.After(filtered[j].LastTimestamp.Time)
	})

	if len(filtered) > 10 {
		filtered = filtered[:10]
	}

	var lines []string
	for _, event := range filtered {
		lines = append(lines, fmt.Sprintf("%s %s: %s", event.Type, event.Reason, event.Message))
	}

	return lines, nil
}