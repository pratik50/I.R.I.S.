package controller

import (
    "sync"
    "time"

    ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
    "github.com/pratik50/iris/clients"
)

// IrisReconciler type
type IrisReconciler struct {
    ctrlclient.Client
    Prometheus       *clients.PrometheusClient
    Loki             *clients.LokiClient
    ArgoCD           *clients.ArgoCDClient
    AI               *clients.AIClient
    RollbackCooldown time.Duration
    LastRollback     map[string]time.Time
    mu               sync.Mutex
}