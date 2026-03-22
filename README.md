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

- headscale ≥ 0.28 with `policy.mode: database` in its config
- headscale gRPC accessible (insecure or TLS) from where headtotails runs

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

Create a Kubernetes `Secret` with the OAuth credentials, then point the operator at headtotails instead of `api.tailscale.com`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: operator-oauth
  namespace: tailscale
stringData:
  client_id: "my-operator"
  client_secret: "my-secret"
```

Set the operator's `--apiserver-proxy-addr` (or equivalent env var) to `http://headtotails.headscale.svc:8080`.

See [`deploy/kustomize/`](deploy/kustomize/) for a complete example.

## Kubernetes deployment (Kustomize)

```bash
# Edit deploy/kustomize/overlays/production/secret.env first
kubectl apply -k deploy/kustomize/overlays/production
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
