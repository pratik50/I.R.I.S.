package main

import (
	  "os"
    "time"

    "github.com/joho/godotenv"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/log/zap"

    "github.com/pratik50/iris/clients"
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
	prometheusURL := os.Getenv("PROMETHEUS_URL")
	var prometheusClient *clients.PrometheusClient
	if prometheusURL == "" {
		logger.Info("⚠️ Prometheus disabled: PROMETHEUS_URL missing. Metrics collection will be skipped.")
	} else {
		prometheusClient = clients.NewPrometheusClient(prometheusURL)
		logger.Info("📡 Prometheus client ready", "url", prometheusURL)
	}

	// Loki client 
	lokiURL := os.Getenv("LOKI_URL")
	var lokiClient *clients.LokiClient
	if lokiURL == "" {
		logger.Info("⚠️ Loki disabled: LOKI_URL missing. Log collection will be skipped.")
	} else {
		lokiClient = clients.NewLokiClient(lokiURL)
		logger.Info("📋 Loki client ready", "url", lokiURL)
	}

	// ArgoCD client
	argoToken := os.Getenv("ARGOCD_TOKEN")
	argoURL := os.Getenv("ARGOCD_URL")
	if argoURL == "" { 
		logger.Info("⏭️ ArgoCD disabled: ARGOCD_URL missing")
		return
	}
	
	var argoClient *clients.ArgoCDClient
	if argoToken == "" {
		logger.Info("⏭️ ArgoCD disabled: ARGOCD_TOKEN missing")
	} else {
		argoClient = clients.NewArgoCDClient(argoURL, argoToken)
		logger.Info("🔄 ArgoCD client ready", "url", argoURL)
	}

	// AI client
	groqKey := os.Getenv("GROQ_API_KEY")
	if groqKey == "" {
		logger.Error(nil, "GROQ_API_KEY env variable missing!")
		os.Exit(1)
	}
	aiClient := clients.NewAIClient(groqKey)
	logger.Info("🤖 AI client ready", "model", "llama-3.1-8b-instant")

	// Slack client (NEW)
    slackEnabled := os.Getenv("SLACK_ENABLED") == "true"
    slackToken := os.Getenv("SLACK_BOT_TOKEN")
    slackChannel := os.Getenv("SLACK_CHANNEL_ID")
    
    var slackClient *clients.SlackClient
    if slackEnabled {
        if slackToken == "" || slackChannel == "" {
            logger.Error(nil, "SLACK_BOT_TOKEN and SLACK_CHANNEL_ID required when SLACK_ENABLED=true")
            os.Exit(1)
        }
        slackClient = clients.NewSlackClient(slackToken, slackChannel, slackEnabled)
        logger.Info("💬 Slack client ready", "channel", slackChannel)
        
        // Send startup message
        if err := slackClient.SendMessage("🚀 I.R.I.S. is now online and watching deployments!"); err != nil {
            logger.Error(err, "Failed to send Slack startup message")
        }
    } else {
        logger.Info("⏭️ Slack disabled — set SLACK_ENABLED=true to enable")
    }

	// IRIS controller
	if err := (&controller.IrisReconciler{
		Client:           mgr.GetClient(),
		Prometheus:       prometheusClient,
		Loki:             lokiClient,
		ArgoCD:           argoClient, 
		AI:               aiClient, 
 		Slack:            slackClient,
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