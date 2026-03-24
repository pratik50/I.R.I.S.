package main

import (
	"os"
	"time"

	"github.com/joho/godotenv"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/pratik50/iris/controller"
)

func main() {

	// load .env file 
	_ = godotenv.Load()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	logger := ctrl.Log.WithName("iris")
	logger.Info("🚀 IRIS starting up...")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{})
	if err != nil {
		logger.Error(err, "Failed to create manager")
		os.Exit(1)
	}

	// Prometheus client
	prometheusClient := controller.NewPrometheusClient("http://localhost:9090")
	logger.Info("📡 Prometheus client ready", "url", "http://localhost:9090")

	// Loki client
	lokiClient := controller.NewLokiClient("http://localhost:3100")
	logger.Info("📋 Loki client ready", "url", "http://localhost:3100")

	// ArgoCD client — nayi addition
	argoToken := os.Getenv("ARGOCD_TOKEN")
	argoURL := os.Getenv("ARGOCD_URL")
	if argoURL == "" {
		argoURL = "http://localhost:8080"
	}
	var argoClient *controller.ArgoCDClient
	if argoToken == "" {
		logger.Info("⏭️ ArgoCD disabled — ARGOCD_TOKEN missing")
	} else {
		argoClient = controller.NewArgoCDClient(argoURL, argoToken)
		logger.Info("🔄 ArgoCD client ready", "url", argoURL)
	}

	// AI client
	groqKey := os.Getenv("GROQ_API_KEY")
	if groqKey == "" {
		logger.Error(nil, "GROQ_API_KEY env variable missing!")
		os.Exit(1)
	}
	aiClient := controller.NewAIClient(groqKey)
	logger.Info("🤖 AI client ready", "model", "llama-3.1-8b-instant")

	// IRIS controller
	if err := (&controller.IrisReconciler{
		Client:           mgr.GetClient(),
		Prometheus:       prometheusClient,
		Loki:             lokiClient,
		ArgoCD:           argoClient, // ← nayi addition
		AI:               aiClient, // ← nayi addition
		RollbackCooldown: 2 * time.Minute,
		LastRollback:     make(map[string]time.Time),
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