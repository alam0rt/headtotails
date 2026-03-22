# OAuth in headtotails

## Intent and expectations

This project implements OAuth primarily for compatibility with Tailscale-style management clients.
People reading [Tailscale OAuth clients docs](https://tailscale.com/docs/features/oauth-clients) should expect similar high-level flow in `headtotails` (client credentials -> short-lived bearer token -> call `/api/v2/*`), but not full feature parity.

## What Tailscale OAuth clients (SaaS) imply

In Tailscale SaaS, OAuth clients are centered around:

- OAuth 2.0 client credentials flow
- 1-hour access tokens
- scope-driven API authorization
- optional `scope` and `tags` parameters on token requests
- lifecycle controls in trust credentials (create/revoke/delete clients)

That is the "spirit" many users bring when they see "OAuth support."

## What headtotails supports today

Current behavior is intentionally minimal:

- token endpoints at `/oauth/token` and `/api/v2/oauth/token`
- `grant_type=client_credentials` only
- static credential check via `OAUTH_CLIENT_ID` and `OAUTH_CLIENT_SECRET`
- short-lived bearer tokens (~1 hour)
- bearer validation for `/api/v2/*`
- direct `HEADSCALE_API_KEY` acceptance as an alternate auth path

## OAuth flows and use cases

### Flow A: Current default flow (implemented today)

```text
Client (operator / SDK / automation)
  | 1) POST /oauth/token
  |    grant_type=client_credentials
  |    client_id + client_secret
  v
headtotails
  | 2) Validates static env credentials
  |    (OAUTH_CLIENT_ID/OAUTH_CLIENT_SECRET)
  | 3) Issues 1h bearer token (in-memory token store)
  v
Client
  | 4) Calls /api/v2/... with Authorization: Bearer <token>
  v
headtotails
  | 5) Validates token exists + not expired
  | 6) Translates REST request to headscale gRPC
  v
headscale
```

### Flow B: Direct API key flow (implemented today)

```text
Client
  | Authorization: Bearer <HEADSCALE_API_KEY>
  | (or Basic auth username=<HEADSCALE_API_KEY>)
  v
headtotails
  | Accepts key directly, skips oauth token lookup
  | Proxies request to headscale gRPC
  v
headscale
```

### Flow C: Federated provider flow (target design; not implemented yet)

```text
Client
  | 1) Gets token from Keycloak / tsidp / other IdP
  |    (client credentials, device flow, auth code, etc.)
  v
External IdP
  | 2) Issues token (JWT or opaque)
  v
Client
  | 3) Calls headtotails /api/v2/... with bearer token
  v
headtotails
  | 4) Validates token via JWKS or introspection
  | 5) Applies claim/scope -> permission mapping
  | 6) Proxies allowed requests to headscale gRPC
  v
headscale
```

### Use cases matrix

| Use case | Works today | Auth source | Notes |
|---|---|---|---|
| Tailscale operator-compatible automation | Yes | headtotails local OAuth | Main reason `/oauth/token` exists in this repo |
| Simple scripts/CI with single shared secret | Yes | `HEADSCALE_API_KEY` | Fast path, fewer moving parts |
| Human SSO login for `tailscale up` | Yes (separate plane) | headscale OIDC (e.g., Keycloak) | Not the same as management API OAuth |
| Federated API tokens from Keycloak | Not yet | External IdP | Needs headtotails token validation + authz mapping |
| Federated API tokens from tsidp | Not yet | External IdP | Same requirement as Keycloak |

### Quick decision tree

```text
What are you trying to do?
|
|-- Run Tailscale operator / SDK against headscale today
|     -> Use headtotails local OAuth:
|        POST /oauth/token -> Bearer token -> /api/v2/*
|
|-- Run simple automation with one shared secret
|     -> Use HEADSCALE_API_KEY directly (Bearer or Basic username)
|
|-- Let humans sign in interactively (SSO)
|     -> Use headscale OIDC (Keycloak/etc) for device/user login
|        (separate from management API OAuth)
|
`-- Use Keycloak/tsidp-issued API tokens for /api/v2/*
      -> Not supported today
      -> Requires upstream token validation + authz mapping in headtotails
```

## Where headtotails differs from Tailscale SaaS today

Not currently implemented:

- no scope or tag enforcement model equivalent to Tailscale SaaS OAuth clients
- no trust-credentials style client lifecycle management API
- no external authorization server integration by default (for example, Keycloak JWT/JWKS validation or introspection)
- no persistent token storage across restarts
- no rich RBAC/claim-aware authorization semantics

Because of these gaps, OAuth in this repo should be treated as a compatibility layer, not as a drop-in SaaS-equivalent IAM system.

## OIDC login vs management API OAuth

There are two separate auth planes:

1. Device/user login plane (OIDC): headscale + IdP browser login flow.
2. Management API machine auth plane (OAuth client credentials): callers obtain token and call `/api/v2/*`.

Using Keycloak for headscale user login does not automatically make headtotails management API tokens Keycloak-issued. That requires explicit upstream OAuth provider support in headtotails.

## Upstream provider direction (requested)

Relevant request: [tailscale/tailscale#14926](https://github.com/tailscale/tailscale/issues/14926).

The issue argues that long-lived client secrets are cumbersome and asks for an end-user-oriented, federated login flow that yields short-lived API tokens (instead of static client-secret-centric workflows).

For headtotails, this is a useful direction signal:

- support an upstream OAuth/OIDC provider path for token issuance and validation
- preserve client-credentials compatibility for existing automation
- introduce clearer scope/claim-to-permission mapping so OAuth means more than "token vending"

Until that work exists, callers should assume headtotails OAuth is compatibility-focused and intentionally simpler than Tailscale SaaS OAuth clients.
