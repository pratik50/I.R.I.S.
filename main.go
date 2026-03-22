package main

import (
	"os"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/pratik50/iris/controller"
)

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	logger := ctrl.Log.WithName("iris")
	logger.Info("🚀 IRIS starting up...")

	// Manager banao
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{})
	if err != nil {
		logger.Error(err, "Failed to create manager")
		os.Exit(1)
	}

	// Prometheus client banao
	// Port forward chal raha hai — localhost:9090
	prometheusClient := controller.NewPrometheusClient(
		"http://localhost:9090",
	)
	logger.Info("📡 Prometheus client ready", "url", "http://localhost:9090")
	
	lokiClient := controller.NewLokiClient(
		"http://localhost:3100",
	)
	logger.Info("📡 Loki client ready", "url", "http://localhost:3100")

	// IRIS controller register karo — Prometheus saath mein do
	if err := (&controller.IrisReconciler{
		Client:     mgr.GetClient(),
		Prometheus: prometheusClient, // ← nayi addition
		Loki:       lokiClient,       // ← nayi addition
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "Failed to setup controller")
		os.Exit(1)
	}

	logger.Info("👁️  Watching deployments...")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error(err, "Manager failed")
		os.Exit(1)
	}
}