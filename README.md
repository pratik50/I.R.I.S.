# IRIS

IRIS is a Kubernetes controller that watches Deployments and triggers an ArgoCD rollback when a rollout is unhealthy. It also collects metrics, logs, and events to help explain failures.

## What You Need

- Kubernetes cluster + `kubectl`
- ArgoCD installed in the cluster
- (Optional) `argocd` CLI for token generation
- Go installed (to run IRIS locally)

## Quick Start (Minimal)

### 1) Create IRIS account + RBAC in ArgoCD

Apply these two config maps in the `argocd` namespace.

**argocd-cm.yaml**

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  accounts.iris: apiKey
```

**argocd-rbac-cm.yaml**

```
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

Apply:

```
kubectl apply -f rbac/argocd-cm.yaml
kubectl apply -f rbac/argocd-rbac-cm.yaml
```

### 2) Generate IRIS token

Login to ArgoCD and generate a token:

```
argocd login <ARGOCD_HOST> --username admin --password <admin-pass> --insecure
argocd account generate-token --account iris
```

Copy the token output.

### 3) Configure IRIS

Create a `.env` file in this repo:

```
ARGOCD_URL=https://localhost:8084
ARGOCD_TOKEN=<paste-token-here>
```

If you are using port-forwarding, run this in another terminal:

```
kubectl -n argocd port-forward svc/argocd-server 8084:443
```

### 4) Add annotation to your app Deployment

IRIS triggers rollback only when this annotation exists:

```
metadata:
  annotations:
    iris.argoproj.io/app: <argocd-app-name>
```

`<argocd-app-name>` must match the ArgoCD Application name.

### 5) Run IRIS

```
go run main.go
```

IRIS will now watch Deployments and trigger rollback on failure.

## Where Do My App Manifests Go?

Your application stays in your own repo and cluster. IRIS does not require your app YAML here. You only need to add the annotation above to your Deployment.

## Notes

- IRIS only rolls back apps in the `default` namespace (current behavior).
- Rollback has a cooldown to prevent repeated loops during rollout.

## Troubleshooting

- If rollback does not trigger, check that the Deployment has the annotation and that `ARGOCD_TOKEN` is set.
- If ArgoCD API calls fail, verify `ARGOCD_URL` and port-forwarding.
