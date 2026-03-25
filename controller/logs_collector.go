package controller

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// collectLogs fetches error logs or recent logs from Loki
func (r *IrisReconciler) collectLogs(ctx context.Context, name, namespace string) ([]string, error) {
	logger := log.FromContext(ctx)

	// First try to get error logs
	logs, err := r.Loki.FetchErrorLogs(ctx, name, namespace, 5*time.Minute)
	if err != nil {
		logger.Error(err, "Loki error logs fetch failed")
		// Continue to try recent logs
	} else if len(logs) > 0 {
		logger.Info("📋 ERROR LOGS COLLECTED", "deployment", name, "total_logs", len(logs))
		for i, logLine := range logs {
			logger.Info(fmt.Sprintf("  LOG %d: %s", i+1, logLine))
		}
		return logs, nil
	}

	// No error logs, fetch recent logs
	logger.Info("📋 NO ERROR LOGS, fetching recent logs", "deployment", name)
	recentLogs, err := r.Loki.FetchRecentLogs(ctx, name, namespace, 5*time.Minute)
	if err != nil {
		logger.Error(err, "Loki recent logs fetch failed")
		return nil, err
	}

	if len(recentLogs) > 0 {
		logger.Info("📋 RECENT LOGS COLLECTED", "deployment", name, "total_logs", len(recentLogs))
		for i, logLine := range recentLogs {
			logger.Info(fmt.Sprintf("  LOG %d: %s", i+1, logLine))
		}
	} else {
		logger.Info("📋 NO RECENT LOGS FOUND", "deployment", name)
	}
	return recentLogs, nil
}