# Stateless IdP Integration Plan

## Goal

Allow `headscale + headtotails + pluggable upstream providers` to expose the full Tailscale management API surface from `tailscale-api.json`, while keeping `headtotails` stateless and preserving backward-compatible defaults.

In this model:

- Identity requests are resolved live through an IdP provider adapter (for example Keycloak, in-memory).
- Tailscale management requests that map to headscale capabilities remain translated via gRPC.
- `headtotails` remains stateless across replicas.

## Non-goals

- Building a local invite/user database in `headtotails`.
- Replacing headscale as the source of truth for devices and network state.
- Making correctness depend on in-memory cache/state.

## Upstream API baseline

Source of truth: `~/Downloads/tailscale-api.json` (OpenAPI 3.1).

- Paths: `56`
- Operations: `85` (GET/POST/PUT/PATCH/DELETE)
- Major families:
  - `tailnet/*` (ACL, keys, DNS, logging, services, contacts, settings, users, user-invites, webhooks)
  - `device/*` + `device-invites/*`
  - `users/*`
  - `webhooks/*` (top-level resource routes)
  - `posture/integrations/*` (top-level resource routes)

Plan requirement: we maintain an explicit per-operation ownership and never silently fall back to incorrect semantics.

## High-level architecture

```text
Caller (operator / sdk / automation)
  |
  |  HTTP /api/v2/*
  v
headtotails
  |-- identity plane routes --> IdentityProvider (Keycloak, in-memory, ...)
  |
  `-- tailnet/device plane --> headscale gRPC
```

Two independent upstreams:

1. IdP adapter: user metadata, verification status, invite semantics (if provider supports it).
2. headscale gRPC: devices, keys, ACL policy, and existing node/user lifecycle operations.

## Endpoint ownership model

Use explicit ownership so behavior is predictable and complete:

- **Provider-backed (IdP):**
  - `/tailnet/{t}/users` (optionally merged with headscale user list)
  - `/users/{id}` (if `id` is provider user id or mapped id)
  - `/tailnet/{t}/user-invites*` (only if provider can support equivalent behavior)
  - any additional user metadata fields (`email`, `emailVerified`)
- **Headscale-backed (gRPC):**
  - `/tailnet/{t}/devices`, `/device/{id}*`
  - `/tailnet/{t}/keys*`
  - `/tailnet/{t}/acl*`
  - existing mutations already implemented today
- **Additional provider-backed (non-IdP control-plane adapters):**
  - `/tailnet/{t}/dns*`
  - `/tailnet/{t}/logging*`
  - `/tailnet/{t}/contacts*`
  - `/tailnet/{t}/settings*`
  - `/tailnet/{t}/services*`
  - `/tailnet/{t}/webhooks` plus top-level `/webhooks/{endpointId}*`
  - `/tailnet/{t}/posture/integrations` plus top-level `/posture/integrations/{id}`
  - `/tailnet/{t}/aws-external-id*`
  - `/device-invites*`
- **Remain `501`:**
  - endpoints with no configured upstream capability.
  - any IdP-owned operation when external IdP is disabled.

### Route-shape compatibility requirements

The plan must match upstream path templates exactly. In particular:

- `user-invites` item routes are top-level (`/user-invites/{userInviteId}*`), not nested under `/tailnet/{tailnet}`.
- `webhooks` item routes are top-level (`/webhooks/{endpointId}*`), while collection routes remain tailnet-scoped.
- `posture integrations` item route is top-level (`/posture/integrations/{id}`).

Keep current aliases temporarily only behind compatibility flags; canonical behavior follows upstream spec.

## Full SaaS feasibility statement

`headscale + headtotails + IdP` alone is sufficient for identity/auth + core device/key/ACL APIs, but **not sufficient** for the full management API surface.

To host the entire SaaS API surface, `headtotails` also needs additional stateless adapters for non-identity domains (DNS/settings/logging/webhooks/services/posture/contacts/aws-external-id/device-invites), each with durable upstream backing.

Without those adapters, those operations must return deterministic `501` with reason strings.

## Identity provider abstraction

Add an interface in `internal/identity` and keep handlers unaware of provider specifics.

```go
type Provider interface {
    Name() string

    // Users
    ListUsers(ctx context.Context, f UserFilter) ([]User, error)
    GetUser(ctx context.Context, id string) (*User, error)

    // Invites (provider-defined semantics)
    ListInvites(ctx context.Context) ([]Invite, error)
    GetInvite(ctx context.Context, id string) (*Invite, error)
    CreateInvite(ctx context.Context, req CreateInviteRequest) (*Invite, error)
    DeleteInvite(ctx context.Context, id string) error
    ResendInvite(ctx context.Context, id string) error

    // Authn/authz support
    ValidateAccessToken(ctx context.Context, token string) (*Principal, error)
}
```

Implementation targets:

- `keycloak` provider:
  - users from Keycloak admin/user endpoints.
  - `emailVerified` from provider user attributes/claims.
  - invite operations mapped to supported Keycloak flow, else explicit `ErrNotSupported`.
- `inmemory` provider:
  - deterministic mock behavior for tests and local dev.

## Stateless authentication design

Current `headtotails` auth uses locally issued HMAC tokens with in-memory token store. To stay stateless across replicas, add an IdP-backed path:

- Accept bearer tokens issued by external IdP (Keycloak).
- Validate JWT via issuer + JWKS (or introspection for opaque tokens).
- Build request principal from claims (`sub`, `email`, `email_verified`, groups/roles).
- Authorize request via claim/group mapping rules.

### Token exchange compatibility

Add `/api/v2/oauth/token-exchange` support so clients using identity federation can work against `headtotails`.

- Input: provider token (`jwt`) + configured client id.
- Validation: via provider adapter.
- Output options:
  1. pass-through provider token if suitable for API auth, or
  2. short-lived signed token accepted by stateless validator (JWT preferred over in-memory token id).

Recommended for stateless operation: signed JWT access tokens validated by all replicas without shared storage.

## Data mapping strategy

Define a normalized internal identity model regardless of provider:

- `id` (stable provider subject key)
- `loginName`
- `displayName`
- `email`
- `emailVerified`
- `status`
- `role`

Then map to current API response model, extending with optional fields where needed.

Suggested additions to `internal/model/user.go`:

- `Email string \`json:"email,omitempty"\``
- `EmailVerified *bool \`json:"emailVerified,omitempty"\``
- `IdentityProvider string \`json:"identityProvider,omitempty"\``

If strict compatibility is required, expose extra fields behind config or separate endpoint.

## Invite behavior in a stateless system

Invites can only be supported if an upstream provider has durable invite state.

- For Keycloak provider, implement invites only if mapped cleanly to Keycloak-native invitation/onboarding mechanism.
- If provider cannot model the needed operations (`resend`, `delete`, lifecycle states), return `501` with a clear reason.
- Do not emulate invite persistence in `headtotails` memory (breaks multi-replica correctness).

## Configuration additions

Add env config for provider selection and validation:

- `IDENTITY_PROVIDER=keycloak|inmemory|none`
- `IDP_ISSUER_URL=...`
- `IDP_JWKS_URL=...` (optional when derivable from issuer)
- `IDP_AUDIENCE=...`
- `IDP_CLIENT_ID=...`
- `IDP_CLIENT_SECRET=...` (if needed for provider API calls)
- `IDP_TIMEOUT=5s`
- `IDP_INSECURE_SKIP_VERIFY=false` (dev only)

Keep existing OAuth envs for backward compatibility during migration.

### Default behavior and fallback

- Default remains existing behavior (`IDENTITY_PROVIDER=none` unless explicitly configured).
- When `IDENTITY_PROVIDER=none`:
  - existing headscale-backed routes behave exactly as today.
  - IdP-owned routes return `501 Not Implemented` with clear reason (`external identity provider not configured`).
- When `IDENTITY_PROVIDER!=none` but provider is unreachable:
  - identity routes return `503` (upstream unavailable), not `501`.

## Implementation phases

### Phase 0: Spec lock + contract harness

- Generate operation inventory from upstream OpenAPI and commit as test fixtures.
- Add contract tests asserting path + method parity for all known operations.
- Add explicit expected mode per operation: `headscale`, `idp`, `other-provider`, or `501`.

### Phase 1: Foundation

- Add provider interface + wiring in config/bootstrap.
- Add `inmemory` provider for tests and local development.
- Add Keycloak token validation module (JWKS/introspection).
- Add auth mode switch: local HMAC (existing) vs provider-validated bearer.

### Phase 2: Identity reads

- Route user list/get to provider adapter.
- Extend user translation model with optional email verification fields.
- Add integration tests for users with both providers.

### Phase 3: Federation compatibility

- Implement `/api/v2/oauth/token-exchange`.
- Add tests with `tailscale-client-go-v2` identity federation flow.
- Document failure modes for expired/invalid provider tokens.

### Phase 4: Invites

- Implement provider capability checks.
- For unsupported providers: deterministic `501` responses.
- For supported providers: implement list/create/get/delete/resend.
- Add contract tests to keep response shape stable.

### Phase 5: Non-identity provider adapters

- Introduce provider interfaces for non-identity domains (DNS/settings/logging/webhooks/services/posture/contacts/aws-external-id/device-invites).
- Implement at least one durable upstream per domain (or keep deterministic `501` policy).
- Ensure canonical top-level routes (`/webhooks/{id}`, `/user-invites/{id}`, `/posture/integrations/{id}`) are handled.

### Phase 6: Hardening

- Add request-level timeouts/circuit breaking for provider calls.
- Add Prometheus metrics and structured logs for provider latency/errors.
- Add retry policy for idempotent provider reads only.

## Testing plan

- Unit tests:
  - provider interface conformance tests.
  - token validation edge cases (`exp`, `nbf`, issuer mismatch, key rotation).
  - mapping tests for `emailVerified` and role/group mapping.
- Integration tests:
  - `inmemory` provider end-to-end.
  - Keycloak container-backed tests (or mocked JWKS + admin API).
  - mixed routes: users from provider, devices from headscale in the same run.
- Compatibility tests:
  - `tailscale-client-go-v2` OAuth/client path (existing).
  - identity federation token exchange path (new).

## Operational concerns

- Define fail behavior when provider is unavailable:
  - identity routes: `503` (upstream unavailable).
  - headscale routes: continue to function if provider is not required for authn.
- Add cache only if needed, and keep it bounded/ephemeral with TTL; correctness must not depend on cache.
- Prefer strict timeout defaults (for example 2-5 seconds) to avoid request pileups.

## Risks and mitigations

- **Risk:** Keycloak invite semantics do not match Tailscale invite endpoints.
  - **Mitigation:** capability-based support, explicit `501` for unsupported operations.
- **Risk:** JWT validation drift across replicas.
  - **Mitigation:** shared issuer/JWKS config, deterministic validator implementation, key-rotation tests.
- **Risk:** Breaking existing clients using current local OAuth behavior.
  - **Mitigation:** feature flags and dual-mode rollout before default switch.

## Rollout strategy

1. Ship provider abstraction behind feature flag, default unchanged.
2. Enable provider-backed auth in non-production.
3. Enable provider-backed user endpoints while keeping device/key/ACL on gRPC path.
4. Add token-exchange endpoint and validate with real clients.
5. Promote provider-backed mode to default when stable.

## Success criteria

- Multi-replica deployment works without sticky sessions for provider-authenticated requests.
- `/users` and `/users/{id}` can include `emailVerified` from provider with no local DB.
- Device/key/ACL flows remain stable and continue to use headscale gRPC.
- Invite endpoints either work via provider-backed durable semantics or return clear, deterministic `501`.
- Every upstream OpenAPI operation is either:
  - implemented with compatible request/response semantics, or
  - explicitly marked as deterministic `501` by policy in `IDENTITY_PROVIDER=none` mode.
