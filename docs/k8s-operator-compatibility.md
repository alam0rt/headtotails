# Tailscale `cmd/k8s-operator` vs `headtotails` Compatibility

This document compares current upstream `tailscale/tailscale` `cmd/k8s-operator` behavior with what `headtotails` currently implements when used against a self-hosted Headscale control plane.

## Summary

`headtotails` is compatible with the core operator path (OAuth client credentials, key creation, device get/delete), but there are important gaps for full mainline operator compatibility (notably VIP services and token exchange).

## Compatibility Table

| Area | Upstream operator expectation (`cmd/k8s-operator`) | `headtotails` status | Impact | Recommended action |
|---|---|---|---|---|
| OAuth token endpoint (client credentials) | `POST /api/v2/oauth/token` | Implemented | Works for default static credential flow | None |
| Legacy OAuth endpoint | `POST /oauth/token` (legacy/compat path) | Implemented | Works for clients using legacy path | None |
| OAuth token exchange (WIF mode) | `POST /api/v2/oauth/token-exchange` | Not implemented | Workload identity federation mode fails | Add token exchange endpoint or avoid WIF mode |
| Auth key management | `GET/POST /api/v2/tailnet/{t}/keys`, `GET/DELETE /keys/{id}` | Implemented | Operator key generation works | None |
| Device get/delete | `GET/DELETE /api/v2/device/{id}` | Implemented | Cleanup and status lookup works | None |
| Device list | `GET /api/v2/tailnet/{t}/devices` | Implemented | Tailnet/device reads work | None |
| VIP Services API | `GET/PUT/DELETE /api/v2/tailnet/{t}/services*` | Returns `501` by design | Tailnet readiness and VIP-service-related features can fail | Implement services endpoints or avoid features requiring VIP services |
| Tailnet CR readiness checks | Reconciler validates OAuth by calling devices + keys + VIP services list | Partially compatible (fails on services list) | Tailnet may stay not-ready in upstream reconciler | Implement VIP services list semantics or skip Tailnet CR workflow |
| Operator base URL env | `OPERATOR_LOGIN_SERVER` sets control/API base | Repo docs currently mention `TAILSCALE_API_URL` patch | Misconfiguration risk with current upstream operator | Update docs/manifests to `OPERATOR_LOGIN_SERVER` |
| Proxy login-server propagation | Proxies must use Headscale login server | Compatible via Tailnet `loginUrl` and ProxyClass `TS_EXTRA_ARGS` | Proxy registration can work against Headscale | Keep existing overlay pattern |
| Multi-replica API auth behavior | Any operator/API client may hit any pod | In-memory OAuth token store per replica | Possible intermittent 401 without sticky sessions/shared validator | Use single replica, sticky sessions, or stateless shared token validation |

## Endpoint-Level Diff (Operator-Critical)

| Endpoint | Needed by upstream operator | `headtotails` |
|---|---|---|
| `POST /api/v2/oauth/token` | Yes | `200` supported |
| `POST /api/v2/oauth/token-exchange` | Optional (WIF mode) | `501`/missing |
| `GET /api/v2/tailnet/{t}/keys` | Yes | `200` supported |
| `POST /api/v2/tailnet/{t}/keys` | Yes | `200` supported |
| `GET /api/v2/device/{id}` | Yes | `200` supported |
| `DELETE /api/v2/device/{id}` | Yes | `200` supported |
| `GET /api/v2/tailnet/{t}/services` | Used by reconcilers/readiness and VIP-service flows | `501` not implemented |
| `PUT /api/v2/tailnet/{t}/services/{serviceId}` | Used by VIP-service flows | `501` not implemented |
| `DELETE /api/v2/tailnet/{t}/services/{serviceId}` | Used by VIP-service flows | `501` not implemented |

## Suggested Compatibility Profiles

| Profile | Expected result |
|---|---|
| Core operator usage (keys/devices, static OAuth secret, no WIF, no VIP-service-heavy features) | Generally compatible |
| Full upstream feature parity with current `cmd/k8s-operator` | Not yet compatible |
| WIF/token-exchange deployment | Not compatible until `/oauth/token-exchange` exists |
| Tailnet CR with strict readiness checks against services API | May fail/not-ready until VIP services API behavior is addressed |
