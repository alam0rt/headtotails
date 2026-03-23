# Deploy with Gateway API (No ingress-nginx)

This guide shows how to wire Tailscale operator + headscale + headtotails using **Gateway API** instead of ingress-nginx.

## Why this path

- ingress-nginx is retired/retiring and no longer the recommended long-term path.
- Gateway API is the Kubernetes-recommended replacement for new ingress/routing setups.
- This repo includes a Gateway API overlay at `deploy/kustomize/operator`.

## What gets routed

Use one shared hostname (example: `headscale.example.com`) and split by path:

- `/api/v2/*` -> `headtotails` (management API)
- `/oauth/token` -> `headtotails` (OAuth token endpoint)
- `/` -> `headscale` (control plane endpoints)

The overlay in this repo creates only the **headtotails path route**. Keep your existing headscale route for `/`.

## Prerequisites

- Gateway API CRDs installed (`gateway.networking.k8s.io/v1`).
- A Gateway controller installed (for example: Cilium, Istio, Traefik, Kong, Envoy Gateway).
- A Gateway listener already serving your hostname/TLS.
- Tailscale operator installed.
- `headtotails` deployed via `deploy/kustomize/overlays/production`.

## 1) Deploy headtotails

```bash
kubectl apply -k deploy/kustomize/overlays/production
```

## 2) Create operator OAuth secret

```bash
kubectl create secret generic operator-oauth \
  --namespace tailscale \
  --from-literal=client_id=<OAUTH_CLIENT_ID> \
  --from-literal=client_secret=<OAUTH_CLIENT_SECRET>
```

These values must match `OAUTH_CLIENT_ID` and `OAUTH_CLIENT_SECRET` in `headtotails`.

## 3) Configure the Gateway API overlay

Edit:

- `deploy/kustomize/operator/tailnet.yaml`
  - `spec.loginUrl`
- `deploy/kustomize/operator/proxyclass.yaml`
  - `TS_EXTRA_ARGS` login-server URL
- `deploy/kustomize/operator/httproute-headtotails.yaml`
  - `parentRefs` (Gateway name/namespace/listener)
  - `hostnames`

## 4) Render check (kustomize sanity)

```bash
kubectl kustomize deploy/kustomize/operator
```

If this renders, your overlay syntax is valid.

## 5) Apply the operator wiring

```bash
kubectl apply -k deploy/kustomize/operator
```

## 6) Ensure operator is pointed at your shared URL

Use the same public URL for operator control/API base (example Helm values):

```bash
helm upgrade --install tailscale-operator tailscale/tailscale-operator \
  --namespace tailscale --create-namespace \
  --set-string loginServer="https://headscale.example.com" \
  --set-string oauth.clientId="<OAUTH_CLIENT_ID>" \
  --set-string oauth.clientSecret="<OAUTH_CLIENT_SECRET>"
```

If you deploy the operator via manifests, set equivalent env/value so it uses your shared host.

## Optional example: dedicated Gateway + both routes

If you do not already have a headscale `/` route, create both routes explicitly.

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: headscale-public
  namespace: ingress-system
spec:
  gatewayClassName: <your-class>
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      hostname: headscale.example.com
      tls:
        mode: Terminate
        certificateRefs:
          - name: headscale-tls
```

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: headscale-root
  namespace: headscale
spec:
  parentRefs:
    - name: headscale-public
      namespace: ingress-system
      sectionName: https
  hostnames:
    - headscale.example.com
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: headscale
          port: 8080
```

And keep `httproute-headtotails.yaml` for the more specific `/api/v2` + `/oauth/token` rules.

## Verification checklist

- `kubectl get httproute -A`
- `kubectl get tailnet -n tailscale`
- `kubectl get proxyclass -n tailscale`
- `kubectl logs -n tailscale deploy/operator`
- `kubectl logs -n headscale deploy/headtotails`

Management API check:

```bash
curl -sS -X POST "https://headscale.example.com/api/v2/oauth/token" \
  -d "grant_type=client_credentials&client_id=<id>&client_secret=<secret>"
```
