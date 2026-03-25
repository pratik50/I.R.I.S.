package controller

import (
    "context"

    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/log"
)

// detectFailure returns true if the deployment is in a failed state
func (r *IrisReconciler) detectFailure(deployment *appsv1.Deployment) bool {
	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}
	available := deployment.Status.AvailableReplicas
	progressingCond := getDeploymentCondition(deployment.Status.Conditions, appsv1.DeploymentProgressing)
	availableCond := getDeploymentCondition(deployment.Status.Conditions, appsv1.DeploymentAvailable)

	failure := (desired > 0 && available < desired)
	if progressingCond != nil && progressingCond.Status == corev1.ConditionFalse {
		failure = true
	}
	if availableCond != nil && availableCond.Status == corev1.ConditionFalse {
		failure = true
	}
	return failure
}

// checkCrashLoopBackOff scans pods and returns true if any container is in CrashLoopBackOff
func (r *IrisReconciler) checkCrashLoopBackOff(ctx context.Context, deployment *appsv1.Deployment) (bool, int) {
	logger := log.FromContext(ctx)
	namespace := deployment.Namespace

	shouldForceRollback := false
	totalRestarts := 0

	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, ctrlclient.InNamespace(namespace),
		ctrlclient.MatchingLabels(deployment.Spec.Selector.MatchLabels)); err != nil {
		logger.Error(err, "Failed to list pods for CrashLoopBackOff detection")
		return false, 0
	}

	for _, pod := range podList.Items {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				shouldForceRollback = true
				logger.Info("💥 CrashLoopBackOff detected",
					"pod", pod.Name,
					"restart_count", cs.RestartCount,
					"reason", cs.State.Waiting.Reason)
				break
			}
			if cs.RestartCount > 0 {
				totalRestarts += int(cs.RestartCount)
			}
		}
		if shouldForceRollback {
			break
		}
	}
	return shouldForceRollback, totalRestarts
}

// isRolloutInProgress returns true if a rollout is ongoing but not yet failed
func (r *IrisReconciler) isRolloutInProgress(deployment *appsv1.Deployment, desired int32) bool {
	return deployment.Status.UpdatedReplicas < desired && deployment.Status.UnavailableReplicas == 0
}