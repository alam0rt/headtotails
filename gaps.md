## Tailscale API v2 vs. headtotails coverage

Cross-referencing the [Tailscale API v2 endpoint reference](https://github.com/itunified-io/mcp-tailscale/blob/286bc5862708e7044fbdb019c1fe35114d3747b4/docs/api-reference.md), the `tailscale-client-go-v2` client library, and the headtotails router.

### 🟢 Implemented (backed by headscale gRPC)

| Endpoint | Method | Client method | headscale gRPC |
|---|---|---|---|
| `/oauth/token` | POST | `tailscale.OAuth` auth flow | HMAC token (headtotails-native) |
| `/tailnet/{t}/keys` | GET | `Keys().List()` | `ListPreAuthKeys` |
| `/tailnet/{t}/keys` | POST | `Keys().Create()` | `CreatePreAuthKey` |
| `/tailnet/{t}/keys/{id}` | GET | `Keys().Get()` | `ListPreAuthKeys` (filter) |
| `/tailnet/{t}/keys/{id}` | DELETE | `Keys().Delete()` | `DeletePreAuthKey` |
| `/tailnet/{t}/devices` | GET | `Devices().List()` | `ListNodes` |
| `/device/{id}` | GET | `Devices().Get()` | `GetNode` |
| `/device/{id}` | DELETE | `Devices().Delete()` | `DeleteNode` |
| `/device/{id}/authorized` | POST | `Devices().SetAuthorized()` | `RegisterNode` |
| `/device/{id}/expire` | POST | — | `ExpireNode` |
| `/device/{id}/name` | POST | `Devices().SetName()` | `RenameNode` |
| `/device/{id}/tags` | POST | `Devices().SetTags()` | `SetTags` |
| `/device/{id}/routes` | GET | `Devices().SubnetRoutes()` | `GetNode` (route fields) |
| `/device/{id}/routes` | POST | `Devices().SetSubnetRoutes()` | `SetApprovedRoutes` |
| `/tailnet/{t}/users` | GET | `Users().List()` | `ListUsers` |
| `/users/{id}` | GET | `Users().Get()` | `ListUsers` (filter) |
| `/users/{id}/delete` | POST | — | `DeleteUser` |
| `/tailnet/{t}/acl` | GET | `PolicyFile().Get()` / `.Raw()` | `GetPolicy` |
| `/tailnet/{t}/acl` | POST | `PolicyFile().Set()` | `SetPolicy` |

**19 endpoints implemented**, all backed by headscale v0.28 gRPC calls.

### 🟡 501 Not Implemented — no headscale gRPC backing

All of the following endpoints are registered in the router and return `501 Not Implemented` with a JSON error body explaining why. headscale does not expose equivalent functionality via gRPC, so headtotails cannot translate these requests.

#### Device mutations (SaaS-only)

| Endpoint | Method | Client method | Reason |
|---|---|---|---|
| `/device/{id}/ip` | POST | `Devices().SetIPv4Address()` | Tailscale SaaS IP assignment; headscale assigns IPs at registration |
| `/device/{id}/key` | POST | `Devices().SetKey()` | SaaS key expiry management; no headscale gRPC equivalent |
| `/device/{id}/attributes` | GET | `Devices().GetPostureAttributes()` | Device posture is a SaaS-only feature |
| `/device/{id}/attributes/{key}` | POST | `Devices().SetPostureAttribute()` | Device posture is a SaaS-only feature |
| `/device/{id}/attributes/{key}` | DELETE | `Devices().DeletePostureAttribute()` | Device posture is a SaaS-only feature |
| `/device/{id}/device-invites` | GET/POST | — | SaaS invite flow; headscale has no invite concept |

#### DNS (8 endpoints)

| Endpoint | Method | Client method | Reason |
|---|---|---|---|
| `/tailnet/{t}/dns/nameservers` | GET/POST | `DNS().Nameservers()` / `SetNameservers()` | headscale manages DNS via config file, not a runtime API |
| `/tailnet/{t}/dns/preferences` | GET/POST | `DNS().Preferences()` / `SetPreferences()` | Same |
| `/tailnet/{t}/dns/searchpaths` | GET/POST | `DNS().SearchPaths()` / `SetSearchPaths()` | Same |
| `/tailnet/{t}/dns/configuration` | GET/POST | — | Same |
| `/tailnet/{t}/dns/split-dns` | GET/PATCH/PUT | `DNS().SplitDNS()` / `UpdateSplitDNS()` | Same |

#### Keys — OAuth client & federated identity (SaaS-only key types)

| Endpoint | Method | Client method | Reason |
|---|---|---|---|
| `/tailnet/{t}/keys/{id}` | PUT | `Keys().SetOAuthClient()` / `SetFederatedIdentity()` | OAuth client and federated identity keys are SaaS concepts |

#### ACL sub-endpoints

| Endpoint | Method | Client method | Reason |
|---|---|---|---|
| `/tailnet/{t}/acl/validate` | POST | `PolicyFile().Validate()` | headscale has no dedicated validation RPC |
| `/tailnet/{t}/acl/preview` | POST | — | SaaS policy preview feature |

#### Users (SaaS-only actions)

| Endpoint | Method | Client method | Reason |
|---|---|---|---|
| `/users/{id}/approve` | POST | — | SaaS user approval workflow |
| `/users/{id}/suspend` | POST | — | SaaS user suspension |
| `/users/{id}/restore` | POST | — | SaaS user restoration |
| `/users/{id}/role` | POST | — | SaaS role management |

#### Webhooks (SaaS event delivery infrastructure)

| Endpoint | Method | Client method | Reason |
|---|---|---|---|
| `/tailnet/{t}/webhooks` | GET/POST | `Webhooks().List()` / `Create()` | headscale has no event bus or webhook dispatch |
| `/tailnet/{t}/webhooks/{id}` | GET/PATCH/DELETE | `Webhooks().Get()` / `Update()` / `Delete()` | Same |
| `/tailnet/{t}/webhooks/{id}/rotate` | POST | `Webhooks().RotateSecret()` | Same |
| `/tailnet/{t}/webhooks/{id}/test` | POST | `Webhooks().Test()` | Same |

#### Logging (SaaS log streaming)

| Endpoint | Method | Client method | Reason |
|---|---|---|---|
| `/tailnet/{t}/logging/{type}` | GET/POST | `Logging().*` | SaaS log streaming to third-party SIEM; headscale logs to stdout |
| `/tailnet/{t}/logging/{type}/stream` | GET | `Logging().LogstreamConfiguration()` | Same |

#### Contacts (SaaS billing/security)

| Endpoint | Method | Client method | Reason |
|---|---|---|---|
| `/tailnet/{t}/contacts` | GET | `Contacts().Get()` | Billing/security contacts are a SaaS concept |
| `/tailnet/{t}/contacts/{type}` | PATCH | `Contacts().Update()` | Same |

#### User invites (SaaS invite flow)

| Endpoint | Method | Client method | Reason |
|---|---|---|---|
| `/tailnet/{t}/user-invites` | GET/POST | — | headscale creates users directly; no invite concept |
| `/tailnet/{t}/user-invites/{id}` | GET/DELETE | — | Same |
| `/tailnet/{t}/user-invites/{id}/resend` | POST | — | Same |

#### Device posture integrations (SaaS third-party integrations)

| Endpoint | Method | Client method | Reason |
|---|---|---|---|
| `/tailnet/{t}/posture/integrations` | GET/POST | `DevicePosture().ListIntegrations()` / `CreateIntegration()` | No headscale concept |
| `/tailnet/{t}/posture/integrations/{id}` | GET/PATCH/DELETE | `DevicePosture().GetIntegration()` etc. | Same |

#### VIP services (SaaS networking feature)

| Endpoint | Method | Client method | Reason |
|---|---|---|---|
| `/tailnet/{t}/services` | GET | `VIPServices().List()` | Virtual IP services are a Tailscale SaaS feature |
| `/tailnet/{t}/services/{id}` | PUT/DELETE | `VIPServices().Update()` / `Delete()` | Same |

#### Tailnet settings (SaaS infrastructure)

| Endpoint | Method | Client method | Reason |
|---|---|---|---|
| `/tailnet/{t}/settings` | GET/PATCH | `TailnetSettings().Get()` / `Update()` | Auto-updates, billing, HTTPS certs, regional routing |

---

### Summary

| Category | Total | Implemented | 501 Stub | Reason |
|---|---|---|---|---|
| **OAuth** | 1 | 1 | 0 | |
| **Auth keys** (basic) | 4 | 4 | 0 | |
| **Keys** (OAuth/federated) | 1 | 0 | 1 | SaaS identity management |
| **Devices** (core) | 3 | 3 | 0 | |
| **Devices** (mutations) | 7 | 5 | 2 | authorize/expire/name/tags/routes implemented; ip/key are SaaS-only |
| **Device posture** | 3 | 0 | 3 | SaaS-only |
| **Device invites** | 2 | 0 | 2 | SaaS invite flow |
| **Users** | 6 | 3 | 3 | list/get/delete implemented; approve/suspend/restore/role are SaaS |
| **ACL** | 4 | 2 | 2 | validate/preview have no gRPC backing |
| **DNS** | 11 | 0 | 11 | headscale uses config file |
| **Tailnet settings** | 2 | 0 | 2 | SaaS infrastructure |
| **Contacts** | 2 | 0 | 2 | SaaS billing/security |
| **Webhooks** | 7 | 0 | 7 | No event bus in headscale |
| **Logging** | 3 | 0 | 3 | SaaS log streaming |
| **User invites** | 5 | 0 | 5 | SaaS invite flow |
| **Posture integrations** | 5 | 0 | 5 | SaaS third-party integrations |
| **VIP services** | 3 | 0 | 3 | SaaS networking feature |
| **Totals** | **~69** | **~19** | **~50** | |

The **19 implemented endpoints cover the full Tailscale Kubernetes operator flow** (OAuth, auth keys, device listing) plus the Terraform provider's needs (device mutations, ACL management, user management). The remaining ~50 endpoints are SaaS-only features with no headscale gRPC equivalent.
