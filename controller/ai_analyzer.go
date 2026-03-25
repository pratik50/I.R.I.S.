package controller

import (
    "context"
    "fmt"

    "github.com/pratik50/iris/clients"
    "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *IrisReconciler) analyzeWithAI(ctx context.Context, name, namespace string, metrics *clients.MetricsSummary, logs, events []string) (*clients.AIAnalysis, error) {
    logger := log.FromContext(ctx)

    if r.AI == nil {
        logger.Info("⚠️ AI client not configured")
        return &clients.AIAnalysis{
            RootCause:  "AI client not available",
            RiskScore:  0.3,
            Action:     "alert",
            Suggestion: "Configure Groq API key",
        }, nil
    }

    if metrics == nil {
        logger.Info("⚠️ No metrics available for AI analysis")
        return &clients.AIAnalysis{
            RootCause:  "Metrics unavailable",
            RiskScore:  0.3,
            Action:     "alert",
            Suggestion: "Check Prometheus connectivity",
        }, nil
    }

    logger.Info("🤖 Analyzing with AI...", "deployment", name)
    analysis, err := r.AI.Analyze(ctx, name, namespace, metrics, logs, events)
    if err != nil {
        logger.Error(err, "AI analysis failed")
        return &clients.AIAnalysis{
            RootCause:  "AI analysis failed",
            RiskScore:  0.4,
            Action:     "alert",
            Suggestion: "Manual investigation required - AI service error",
        }, err
    }

    logger.Info("🧠 AI ANALYSIS",
        "deployment", name,
        "root_cause", analysis.RootCause,
        "risk_score", fmt.Sprintf("%.2f", analysis.RiskScore),
        "action", analysis.Action)

    return analysis, nil
}