# IRIS - Intelligent Rollback & Incident Supervisor

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://go.dev)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.35.0-326CE5?style=flat&logo=kubernetes)](https://kubernetes.io)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

I.R.I.S. is an **AI-powered Kubernetes controller** that automatically detects deployment failures, performs intelligent root cause analysis, and triggers instant rollbacks via ArgoCD. It combines real-time monitoring, AI Intellegencia, and automated recovery to minimize downtime.

> **🚧 Status: Under Active Development**  
> Core functionality (failure detection, AI analysis, ArgoCD rollback) is implemented and tested. Slack integration, enhanced observability are currently in progress and many more to go.

## 🦄 What Makes I.R.I.S. Unique

Unlike traditional monitoring tools that only alert you about problems, I.R.I.S. acts as your **24/7 SRE engineer** that:

- **Automatically detects** deployment failures the moment they occur
- **Analyzes root causes** using AI (Groq Llama 3.1) to understand why your deployment failed
- **Makes intelligent decisions** - not every failure needs a rollback; I.R.I.S. evaluates risk scores to decide
- **Triggers instant rollbacks** via ArgoCD, reducing MTTR from hours to seconds
- **Notifies your team** with detailed analysis and recommended fixes

## 🚀 Features

- **🔄 Automatic Rollback** - Triggers ArgoCD rollback when deployment failures are detected
- **🤖 AI-Powered Analysis** - Uses Groq's Llama 3.1 to analyze metrics, logs, and events
- **📊 Multi-Source Data Collection** - Integrates Prometheus metrics and Loki logs
- **⚡ Smart Detection** - Differentiates between first-time crashes and CrashLoopBackOff
- **🛡️ Rollback Cooldown** - Prevents rapid rollback loops during unstable rollouts
- **📁 Modular Architecture** - Clean separation of concerns with client and controller packages
 
>Implementing rignt now:
- 📈 **Grafana Dashboard** - Monitoring and metric analysis dashboad
- 💬 **Slack Notifications** - Real-time notifications for rollbacks and alerts

## 📋 Prerequisites

- Kubernetes cluster (v1.28+) (kind or minikube for local testing)
- ArgoCD installed in the cluster
- Prometheus (for metrics collection)
- Loki (for log collection)
- Go 1.21+
- `argocd` CLI (for token generation: easy to go with)
- Groq API key (for AI analysis) (using grok's llama 3.1 model for now)

## 🏗️ Architecture

```
  ┌─────────────────────────────────────────────────────────────────────────────┐
  │                              I.R.I.S. System                                │
  ├─────────────────────────────────────────────────────────────────────────────┤
  │                                                                             │
  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐   │
  │  │  Prometheus │    │    Loki     │    │   ArgoCD    │    │    Groq     │   │
  │  │   Metrics   │    │    Logs     │    │   Rollback  │    │     AI      │   │
  │  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘    └──────┬──────┘   │
  │         │                  │                  │                  │          │
  │         ▼                  ▼                  ▼                  ▼          │
  │  ┌─────────────────────────────────────────────────────────────────────┐    │
  │  │                      I.R.I.S. Controller                            │    │
  │  │  ┌─────────────────────────────────────────────────────────────┐    │    │
  │  │  │                    Reconciler Loop                          │    │    │
  │  │  │  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐    │    │    │
  │  │  │  │  Failure      │  │  CrashLoop    │  │  AI Analysis  │    │    │    │
  │  │  │  │  Detector     │─▶│  BackOff      │─▶│  & Decision   │    │    │    │
  │  │  │  │               │  │  Detector     │  │               │    │    │    │
  │  │  │  └───────────────┘  └───────────────┘  └───────────────┘    │    │    │
  │  │  └─────────────────────────────────────────────────────────────┘    │    │
  │  └─────────────────────────────────────────────────────────────────────┘    │
  │                                    │                                        │
  │                    ┌───────────────┼───────────────┐                        │
  │                    ▼               ▼               ▼                        │
  │            ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                │
  │            │ Kubernetes  │  │   Slack     │  │   Grafana   │                │
  │            │ Deployment  │  │   Alerts    │  │  Dashboard  │                │
  │            │ (Rollback)  │  │  (working)  │  │  (working)  │                │
  │            └─────────────┘  └─────────────┘  └─────────────┘                │
  └─────────────────────────────────────────────────────────────────────────────┘
```
## 🚀 Installation & Setup

### Step 1: Configure ArgoCD RBAC for IRIS

Create a dedicated service account for IRIS in ArgoCD:

**argocd-cm.yaml**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  accounts.iris: apiKey
```
**argocd-rbac-cm.yaml**
```yaml
  apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
  namespace: argocd
data:
  policy.default: role:readonly
  policy.csv: |
    p, role:iris, applications, get, */*, allow
    p, role:iris, applications, update, */*, allow
    p, role:iris, applications, sync, */*, allow
    p, role:iris, applications, rollback, */*, allow
    g, iris, role:iris
```

Apply these two configurations:

```
kubectl apply -f argocd-cm.yaml
kubectl apply -f argocd-rbac-cm.yaml
```

### Step 2: Generate ArgoCD Token for IRIS

Login to ArgoCD and generate a token:

```
argocd login <ARGOCD_HOST> --username admin --password <admin-pass> --insecure

argocd account generate-token --account iris
```

Copy the generated token.

### Step 3: Configure Environment Variables

Create a .env file in the project root:

```
ARGOCD_URL=your-argocd-url
ARGOCD_TOKEN=<your-token-here>
GROQ_API_KEY=<your-groq-api-key>
PROMETHEUS_URL=your-prometheus-url
LOKI_URL=your-loki-url
```

### Step 4: Add Required Annotation to Your Deployment

IRIS only watches Deployments with this annotation:

```
iris.argoproj.io/app: your-app-name
```

Example:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: your-app
  namespace: default  # ⚠️ Checks for default namespace only right now.
  labels:
    app: your-app
  annotations:
    iris.argoproj.io/app: "your-argocd-app-name"  # Must match ArgoCD Application name
```

### Step 5: Deploy IRIS

Local deployment:
```
go run main.go
```

SOON We will have a helm chart for easy deployment and configuration. Stay tuned!

## 📁 Project Structure

```
iris/
├── main.go                   # Entry point, initializes all clients
├── go.mod                    # Go module dependencies
├── go.sum                    # Dependency checksums
├── .env                      # Environment variables (not committed)
├── controller/               # Core controller logic
│   ├── types.go              # IrisReconciler struct definition
│   ├── helpers.go            # Helper functions (events, conditions)
│   ├── failure_detector.go   # Failure detection logic
│   ├── rollback_handler.go   # Rollback execution & cooldown
│   ├── metrics_collector.go  # Prometheus metrics collection
│   ├── logs_collector.go     # Loki logs collection
│   ├── ai_analyzer.go        # AI analysis with Groq
│   └── iris_reconciler.go    # Main reconciliation loop
└── clients/                  # External service clients
    ├── prometheus_client.go
    ├── loki_client.go
    ├── argocd_client.go
    └── ai_client.go
```
## 🤖 AI Analysis & Risk Scoring
IRIS uses Groq's Llama 3.1 model to analyze failures:

Risk Score	Action
```
≥ 0.5	Auto-rollback triggered
< 0.5	Manual diagnosis required
```
AI considers:

- Container crash status

- Exit codes and error logs

- Kubernetes events

- CPU/Memory metrics

- Pod restart counts 

## ⚙️ Rollback Decision Logic
```
┌─────────────────────────────────────────────────────────────-┐
│                   Rollback Decision Tree                     │
├─────────────────────────────────────────────────────────────-┤
│                                                              │
│  1. Detect Deployment Failure                                │
│         │                                                    │
│         ▼                                                    │
│  2. Check CrashLoopBackOff? ──── YES ───► Force Rollback     │
│         │                                                    │
│         NO                                                   │
│         │                                                    │
│         ▼                                                    │
│  3. Collect Metrics & Logs                                   │
│         │                                                    │
│         ▼                                                    │
│  4. AI Analysis (Groq)                                       │
│         │                                                    │
│         ▼                                                    │
│  5. Risk Score ≥ 0.5? ──────── YES ───► Auto-Rollback        │
│         │                                                    │
│         NO                                                   │
│         │                                                    │
│         ▼                                                    │
│  6. Manual Diagnosis Required (Alert)                        │
│                                                              │
└─────────────────────────────────────────────────────────────-┘
```

## 🔍 Troubleshooting
>Rollback Not Triggering

- Check Deployment has annotation iris.argoproj.io/app: <app-name>

- Verify ARGOCD_TOKEN is set correctly

- Ensure ArgoCD app exists and has history (at least 2 revisions)

- Check cooldown period (default 2 minutes)

>AI Analysis Not Working

- Verify GROQ_API_KEY is set

- Check network connectivity to Groq API

- Ensure metrics are available (Prometheus)

- Review controller logs for AI errors

>Prometheus/Loki Issues

- IRIS continues without metrics/logs but uses fallback values (risk_score = 0.3-0.4)

- ArgoCD Connection Failed
- Verify ARGOCD_URL is accessible from IRIS

- Check token validity: argocd account can-i get applications --account iris

## 📝 Important Notes
- IRIS watches Deployments in Default namespaces but only rolls back those with the required annotation

- Rollback has a cooldown period (configurable) to prevent loops

- Force rollback triggers on any container crash or CrashLoopBackOff

- Rollback is only triggered if the deployment is in a failed state

## 🤝 Contributing
	1.	Fork the repo
	2.	Create a feature branch (git checkout -b feature-name)
	3.	Commit changes (git commit -m "Added new feature")
	4.	Push branch (git push origin feature-name)
	5.	Open a Pull Request 🚀  

## 🆓 End Note  

This project is **open-source and free to use**.  
You are welcome to **modify, distribute, and learn** from it.  

Made with ❤️ by **Pratik Jadhav**  

