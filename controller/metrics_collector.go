package controller

import (
    "context"
    "fmt"

    "github.com/pratik50/iris/clients"
    "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *IrisReconciler) collectMetrics(ctx context.Context, name, namespace string) (*clients.MetricsSummary, error) {
    logger := log.FromContext(ctx)
    metrics, err := r.Prometheus.FetchDeploymentMetrics(ctx, name, namespace)
    if err != nil {
        logger.Error(err, "Prometheus metrics not available")
        return nil, err
    }
    logger.Info("📊 METRICS COLLECTED",
        "deployment", name,
        "available_replicas", metrics.AvailableReplicas,
        "cpu_cores", fmt.Sprintf("%.4f", metrics.CPUUsage),
        "memory_mb", fmt.Sprintf("%.2f MB", metrics.MemoryMB()))
    return metrics, nil
}