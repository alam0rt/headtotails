# Local Kind E2E: headscale + headtotails + Tailscale Operator

This guide provisions a local Kind cluster and deploys:

- `headscale` (control plane)
- `headtotails` (management API shim)
- a small in-cluster router (`control-router`) that sends:
  - `/api/v2/*` and `/oauth/token` to `headtotails`
  - all other paths to `headscale`
- upstream Tailscale Kubernetes operator (Helm chart)

Then it runs a smoke test that creates a `LoadBalancer` Service with `loadBalancerClass: tailscale` and verifies operator-created proxy resources and node registration in headscale.

## Prerequisites

Use the repository flake dev shell so all required tooling is present:

- `kind`
- `kubectl`
- `helm`
- `podman` or `docker`
- `jq`
- `rg`

## One-command flow

```bash
make kind-e2e
```

This runs:

1. `make kind-up`
2. `make kind-deploy`
3. `make kind-smoke-test`

## Step-by-step flow

```bash
make kind-up
make kind-deploy
make kind-smoke-test
```

## Localhost endpoint mode

If you want a host-local URL for other local tools/services, run:

```bash
KIND_PROVIDER=podman make kind-local-up
```

This will:

1. Ensure the Kind cluster exists
2. Deploy headscale + headtotails + operator
3. Start a local port-forward from `localhost:$LOCAL_CONTROL_PORT` to in-cluster `control-router`

Default URL:

```bash
http://127.0.0.1:18080
```

You can then export variables in another terminal, for example:

```bash
export TAILSCALE_API_URL=http://127.0.0.1:18080
export OPERATOR_LOGIN_SERVER=http://127.0.0.1:18080
```

Helpers:

```bash
make kind-local-status
make kind-local-down
```

## Teardown

```bash
make kind-down
```

## Optional environment overrides

You can override defaults for local experimentation:

- `CLUSTER_NAME` (default: `headtotails`)
- `KIND_PROVIDER` (default: `auto`; auto-prefers `podman` if present)
- `KIND_KUBECONFIG` (default: `.kind/<cluster>.kubeconfig`)
- `LOCAL_CONTROL_PORT` (default: `18080`)
- `OAUTH_CLIENT_ID` (default: `kind-operator`)
- `OAUTH_CLIENT_SECRET` (default: `kind-operator-secret`)
- `OAUTH_HMAC_SECRET` (default: `kind-shared-hmac-secret-32-bytes-min`)
- `TAILNET_NAME` (default: `tailscale-operator`)

Example:

```bash
CLUSTER_NAME=ht2 KIND_PROVIDER=podman OAUTH_CLIENT_ID=my-id OAUTH_CLIENT_SECRET=my-secret make kind-e2e
```

## Notes

- This harness intentionally uses static OAuth credentials (not WIF).
- VIP-service-dependent features may still expose current `headtotails` API gaps.
- The smoke test validates core operator behavior for service proxy creation.
