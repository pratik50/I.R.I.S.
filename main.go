package main

import (
	"os"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/pratik50/iris/controller"
)

func main() {
	// Logger setup — terminal pe logs dikhenge
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	logger := ctrl.Log.WithName("iris")
	logger.Info("🚀 IRIS starting up...")

	// Manager = IRIS ka heart
	// Ye K8s se connect karta hai aur controllers run karta hai
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{})
	if err != nil {
		logger.Error(err, "Failed to create manager")
		os.Exit(1)
	}

	// IRIS controller register karo
	if err := (&controller.IrisReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "Failed to setup controller")
		os.Exit(1)
	}

	logger.Info("👁️  Watching deployments...")

	// Start karo — ye forever chalega
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error(err, "Manager failed")
		os.Exit(1)
	}
}
