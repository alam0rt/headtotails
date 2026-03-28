# Example Tailscale Operator manifests for headtotails

These examples show how to exercise **every capability headtotails
supports** when running as the management API shim for the Tailscale
Kubernetes operator against a headscale control plane.

## Prerequisites

| Component | Purpose |
|-----------|---------|
| **headscale** | VPN control plane (running, reachable) |
| **headtotails** | Tailscale API v2 → headscale gRPC translator |
| **tailscale-operator** | Kubernetes operator pointed at headscale via headtotails |

The operator must be configured with `OPERATOR_LOGIN_SERVER` set to your
headscale URL (e.g. `https://hs.example.com`), and the `operator-oauth`
secret must match headtotails' `OAUTH_CLIENT_ID` / `OAUTH_CLIENT_SECRET`.

## What each example exercises

| Example | headtotails endpoints hit |
|---------|--------------------------|
| `00-namespace.yaml` | — (setup) |
| `01-tailnet.yaml` | `POST /api/v2/oauth/token`, `GET/POST /api/v2/tailnet/-/keys` |
| `02-proxyclass.yaml` | — (operator config, no API call) |
| `03-expose-service.yaml` | `POST /tailnet/-/keys`, `POST /device/{id}/tags` |
| `04-expose-ingress.yaml` | `POST /tailnet/-/keys`, `POST /device/{id}/tags` |
| `05-subnet-router.yaml` | `POST /tailnet/-/keys`, `POST /device/{id}/routes` |
| `06-exit-node.yaml` | `POST /tailnet/-/keys`, `POST /device/{id}/routes` |
| `07-acl-policy.sh` | `POST /oauth/token`, `GET/POST /tailnet/-/acl` |
| `08-node-daemonset.yaml` | `POST /oauth/token`, `POST /tailnet/-/keys`, `POST /device/{id}/tags` |

## Apply order

```bash
kubectl apply -f examples/00-namespace.yaml
kubectl apply -f examples/01-tailnet.yaml
kubectl apply -f examples/02-proxyclass.yaml

# Then pick whichever workloads you want:
kubectl apply -f examples/03-expose-service.yaml
kubectl apply -f examples/04-expose-ingress.yaml
kubectl apply -f examples/05-subnet-router.yaml
kubectl apply -f examples/06-exit-node.yaml

# DaemonSet — expose every node onto the tailnet:
kubectl apply -f examples/08-node-daemonset.yaml

# ACL policy (shell script, not a k8s resource):
bash examples/07-acl-policy.sh
```

## Clean up

```bash
kubectl delete -f examples/ --ignore-not-found
```
