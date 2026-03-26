package controller

import (
    "context"
    "fmt"
    "sort"
    "strings"

	appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// getDeploymentCondition returns the condition with the given type
func getDeploymentCondition(conditions []appsv1.DeploymentCondition, condType appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

// fetchDeploymentEvents collects events related to a deployment and its pods
func (r *IrisReconciler) fetchDeploymentEvents(ctx context.Context, deploymentName, namespace string) ([]string, error) {
	var events corev1.EventList
	if err := r.List(ctx, &events, ctrlclient.InNamespace(namespace)); err != nil {
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


// Helper function to get restart count
func (r *IrisReconciler) getRestartCount(ctx context.Context, deployment *appsv1.Deployment) int {
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, ctrlclient.InNamespace(deployment.Namespace),
		ctrlclient.MatchingLabels(deployment.Spec.Selector.MatchLabels)); err != nil {
		return 0
	}
	
	totalRestarts := 0
	for _, pod := range podList.Items {
		for _, cs := range pod.Status.ContainerStatuses {
			totalRestarts += int(cs.RestartCount)
		}
	}
	return totalRestarts
}