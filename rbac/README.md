# 🔴 NOTE: THIS DIRECTORY IS FOR RBAC AND IMPORTANT TO GENERATE TOKENS FOR IRIS TO WORK.

This is required for IRIS to work and generate tokens for rollbacks and give the right permissions.

If you are using port-forwarding, run this in another terminal:

```
kubectl -n argocd port-forward svc/argocd-server 8084:443
```

Then login to ArgoCD and generate a token:

```
argocd login <ARGOCD_HOST> --username admin --password <admin-pass> --insecure
argocd account generate-token --account iris
```

Copy the token output.

Create a `.env` file in this repo:

```
ARGOCD_URL=your-argocd-url
ARGOCD_TOKEN=<paste-token-here>
```

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