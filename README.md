# headtotails

**headtotails** is a lightweight Go service that exposes the [Tailscale REST API v2](https://tailscale.com/api) as a thin translation layer over the [headscale](https://github.com/juanfont/headscale) gRPC API.

It is designed to run **alongside an existing headscale server**, making headscale compatible with tooling that targets the Tailscale control-plane API — primarily the [Tailscale Kubernetes operator](https://github.com/tailscale/tailscale-kubernetes-operator), the Tailscale Terraform provider, and any other client that speaks `api.tailscale.com/api/v2/…`.

```
┌──────────────────────────────┐        ┌──────────────┐        ┌─────────────┐
│  Tailscale k8s operator /    │  REST  │              │  gRPC  │             │
│  Terraform provider / etc.   │───────▶│   headtotails    │───────▶│  headscale  │
└──────────────────────────────┘        │  :8080       │        │  :50443     │
                                        └──────────────┘        └─────────────┘
```

## Features

| Endpoint group | Status |
|---|---|
| `POST /oauth/token` (client credentials) | ✅ |
| `GET/DELETE /api/v2/tailnet/{t}/devices` | ✅ |
| `GET/DELETE /api/v2/device/{id}` | ✅ |
| `GET/POST /api/v2/tailnet/{t}/keys` | ✅ |
| `GET/DELETE /api/v2/tailnet/{t}/keys/{id}` | ✅ |
| `GET/DELETE /api/v2/tailnet/{t}/users` | ✅ |
| `GET/POST /api/v2/tailnet/{t}/acl` | ✅ |
| `/healthz`, `/metrics` | ✅ |
| DNS, webhooks, logging, settings, … | `501 Not Implemented` |

## Prerequisites

- **headscale ≥ 0.28** with `policy.mode: database` in its config (required for
  ACL read/write via gRPC)
- headscale gRPC accessible (insecure or TLS) from where headtotails runs

### Why headtotails exists

The Tailscale Kubernetes operator (and other Tailscale tooling) needs **two
distinct endpoints** to function:

| Plane | Tailscale SaaS | With headscale |
|---|---|---|
| **VPN control plane** — where `tailscaled` registers WireGuard peers | `https://controlplane.tailscale.com` | your headscale URL |
| **Management API** — create auth keys, list devices, manage ACLs | `https://api.tailscale.com` | headtotails |

headscale implements the VPN control plane but not the management REST API.
The Tailscale team [explicitly declined](https://github.com/tailscale/tailscale/pull/11627)
to add headscale-specific code to the operator, suggesting instead that a REST
shim handle the translation — which is exactly what headtotails does.

With an nginx Ingress fronting both services on the same hostname, the operator
can be configured with a **single URL** for both planes:

## Configuration

headtotails is configured entirely via environment variables:

| Variable | Required | Default | Description |
|---|---|---|---|
| `HEADSCALE_ADDR` | ✅ | — | headscale gRPC address, e.g. `headscale:50443` |
| `HEADSCALE_API_KEY` | ✅ | — | headscale API key (`headscale apikeys create`) |
| `OAUTH_CLIENT_ID` | ✅ | — | OAuth client ID issued to callers |
| `OAUTH_CLIENT_SECRET` | ✅ | — | OAuth client secret |
| `OAUTH_HMAC_SECRET` | ✅ | — | 32-byte secret for signing bearer tokens |
| `TAILNET_NAME` | | `-` | Tailnet identifier used in URL paths |
| `LISTEN_ADDR` | | `:8080` | HTTP listen address |
| `TLS_CERT` | | — | Path to TLS certificate (enables HTTPS) |
| `TLS_KEY` | | — | Path to TLS private key |

## Quickstart (Docker)

```bash
# 1. Create a headscale API key
HEADSCALE_API_KEY=$(headscale apikeys create --expiration 8760h)

# 2. Run headtotails
docker run -d \
  -e HEADSCALE_ADDR=headscale:50443 \
  -e HEADSCALE_API_KEY="$HEADSCALE_API_KEY" \
  -e OAUTH_CLIENT_ID=my-operator \
  -e OAUTH_CLIENT_SECRET=my-secret \
  -e OAUTH_HMAC_SECRET=a-32-byte-random-secret-here!!! \
  -p 8080:8080 \
  ghcr.io/alam0rt/headtotails:latest
```

> **Next step:** headtotails is now running and serving the Tailscale management
> API. To complete the setup, configure the Tailscale Kubernetes operator to use
> headtotails — see [Usage with the Tailscale Kubernetes Operator](#usage-with-the-tailscale-kubernetes-operator) below.

## Quickstart (binary)

```bash
export HEADSCALE_ADDR=127.0.0.1:50443
export HEADSCALE_API_KEY=hskey-api-...
export OAUTH_CLIENT_ID=my-operator
export OAUTH_CLIENT_SECRET=my-secret
export OAUTH_HMAC_SECRET=a-32-byte-random-secret-here!!!

./headtotails
# {"time":"...","level":"INFO","msg":"headtotails starting","addr":":8080"}
```

## Usage with the Tailscale Kubernetes Operator

The recommended deployment routes `/api/v2` and `/oauth/token` to headtotails
via an nginx Ingress on the same hostname as headscale. This gives the operator
a **single URL** for both the VPN control plane and the management API.

```
https://headscale.example.com/            → headscale:8080   (VPN control plane)
https://headscale.example.com/api/v2/    → headtotails:8080 (management API)
https://headscale.example.com/oauth/token → headtotails:8080 (OAuth2 tokens)
```

### Step 1 — Deploy headtotails

Create a dedicated headscale user for the operator (do **not** reuse an
OIDC-linked user — keep operator-managed nodes isolated):

```bash
headscale users create tailscale-operator
```

Then deploy headtotails:

```bash
cp deploy/kustomize/overlays/production/secret.env.example \
   deploy/kustomize/overlays/production/secret.env
# Fill in secret.env, then:
kubectl apply -k deploy/kustomize/overlays/production
```

### Step 2 — Apply the operator wiring overlay

Create the `operator-oauth` Secret in the `tailscale` namespace. The
`client_id` and `client_secret` must match `oauth_client_id` and
`oauth_client_secret` from your `secret.env`:

```bash
kubectl create secret generic operator-oauth \
  --namespace tailscale \
  --from-literal=client_id=<OAUTH_CLIENT_ID> \
  --from-literal=client_secret=<OAUTH_CLIENT_SECRET>
```

Then apply the operator overlay (after editing the `headscale.example.com`
placeholder to your actual headscale hostname):

```bash
kubectl apply -k deploy/kustomize/operator
```

This creates:
- A `Tailnet` CR pointing `loginUrl` at your headscale instance
- A `ProxyClass` that injects `--login-server` into every proxy pod the
  operator spawns
- A headtotails-specific `Ingress` that routes `/api/v2` and `/oauth/token`
  to headtotails on the headscale hostname (separate from the Flux-managed
  headscale Ingress, so it does not conflict)

### Step 3 — Patch the operator Deployment

Add `TAILSCALE_API_URL` to the operator's Deployment so it sends management
API calls to headtotails instead of `api.tailscale.com`:

```yaml
env:
  - name: TAILSCALE_API_URL
    value: "https://headscale.example.com"
```

The operator will then `POST /oauth/token` with the credentials from the
`operator-oauth` Secret, receive an HMAC-signed bearer token from headtotails,
and use it for all subsequent `/api/v2/…` calls.

### OIDC / Keycloak interaction

If your headscale is configured with an OIDC provider (e.g. Keycloak), the two
auth flows are **completely independent**:

- **Device registration (OIDC):** human runs `tailscale up` → headscale redirects
  browser to your OIDC provider → user logs in → headscale maps identity to a
  headscale user. headtotails is **not involved**.
- **Operator (OAuth2 client credentials):** operator POSTs `client_credentials`
  to headtotails `/oauth/token` → headtotails validates inline against
  `OAUTH_CLIENT_ID`/`OAUTH_CLIENT_SECRET` → issues HMAC token. **No OIDC
  provider involved.**

See [`deploy/kustomize/operator/`](deploy/kustomize/operator/) for all manifests.

## Kubernetes deployment (Kustomize)

```bash
# 1. Deploy headtotails
cp deploy/kustomize/overlays/production/secret.env.example \
   deploy/kustomize/overlays/production/secret.env
# Edit secret.env, then:
kubectl apply -k deploy/kustomize/overlays/production

# 2. Wire the Tailscale operator (edit headscale.example.com placeholders first)
kubectl apply -k deploy/kustomize/operator
```

## API

### OAuth token

```bash
curl -s -X POST http://localhost:8080/oauth/token \
  -d 'grant_type=client_credentials&client_id=my-operator&client_secret=my-secret'
# {"access_token":"...","token_type":"Bearer","expires_in":3600}
```

### List devices

```bash
TOKEN=<access_token>
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v2/tailnet/-/devices
```

### Create auth key

```bash
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  http://localhost:8080/api/v2/tailnet/-/keys \
  -d '{"capabilities":{"devices":{"create":{"reusable":false,"ephemeral":true,"preauthorized":true}}},"expirySeconds":3600}'
```

### Health check

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

### Prometheus metrics

```bash
curl http://localhost:8080/metrics
```

## Development

```bash
# Enter nix dev shell (includes go, gcc, protobuf tools)
nix develop

# Build
make build

# Unit tests
make test

# Integration tests (requires Docker/Podman)
make integration-test

# Lint
make lint
```

## Testing

```
Unit tests:        go test ./internal/...           (no Docker required)
Integration tests: HEADSCALE_INTEGRATION_TEST=1 go test -v ./integration/...
```

Integration tests spin up headscale 0.28 via Docker/Podman automatically using `dockertest`, build and start headtotails as a subprocess, and run the full API call sequence end-to-end.

## Architecture

```
cmd/headtotails/main.go          binary entry-point, graceful shutdown
internal/
  config/                    env-var config (envconfig)
  headscale/client.go        HeadscaleClient interface + gRPC implementation
  translate/                 headscale proto ↔ Tailscale JSON model
  api/
    router.go                chi router, middleware wiring
    auth.go                  OAuth token endpoint, bearer middleware
    devices.go               /device and /tailnet/{t}/devices handlers
    keys.go                  /tailnet/{t}/keys handlers
    users.go                 /tailnet/{t}/users handlers
    policy.go                /tailnet/{t}/acl handlers
    metrics.go               Prometheus middleware
integration/                 end-to-end tests (dockertest + subprocess)
```

## License

MIT
