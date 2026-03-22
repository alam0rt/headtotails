## Complete Tailscale API v2 vs. headtotails coverage

Cross-referencing the [Tailscale API v2 endpoint reference](https://github.com/itunified-io/mcp-tailscale/blob/286bc5862708e7044fbdb019c1fe35114d3747b4/docs/api-reference.md), the `tailscale-client-go-v2` client library, and your headtotails README:

### 🟢 Implemented in headtotails (safe to test with the Go client)

| Endpoint | Method | Client method | headtotails status |
|---|---|---|---|
| `/oauth/token` | POST | `tailscale.OAuth` auth flow | ✅ |
| `/tailnet/{t}/keys` | GET | `Keys().List()` | ✅ |
| `/tailnet/{t}/keys` | POST | `Keys().Create()` | ✅ |
| `/tailnet/{t}/keys/{id}` | GET | `Keys().Get()` | ✅ |
| `/tailnet/{t}/keys/{id}` | DELETE | `Keys().Delete()` | ✅ |
| `/tailnet/{t}/devices` | GET | `Devices().List()` | ✅ |
| `/device/{id}` | GET | `Devices().Get()` | ✅ |
| `/device/{id}` | DELETE | `Devices().Delete()` | ✅ |
| `/tailnet/{t}/users` | GET | `Users().List()` | ✅ |
| `/tailnet/{t}/acl` | GET | `PolicyFile().Get()` / `.Raw()` | ✅ |
| `/tailnet/{t}/acl` | POST | `PolicyFile().Set()` | ✅ |

### 🔴 Tailscale SaaS-only — no headscale gRPC backing exists

These endpoints are purely Tailscale SaaS infrastructure. headscale does not and will not implement them, so headtotails can only return `501 Not Implemented`.

#### **Per-device mutation sub-resources** (API exists, but headscale lacks equivalent gRPC)

| Endpoint | Method | Client method | Why SaaS-only |
|---|---|---|---|
| `/device/{id}/authorized` | POST | `Devices().SetAuthorized()` | headscale doesn't expose authorization toggle via gRPC in this form |
| `/device/{id}/tags` | POST | `Devices().SetTags()` | headscale manages tags differently (at registration, not via REST mutation) |
| `/device/{id}/routes` | GET | `Devices().SubnetRoutes()` | headscale has route management via CLI/gRPC but not in Tailscale REST shape |
| `/device/{id}/routes` | POST | `Devices().SetSubnetRoutes()` | Same — different gRPC model |
| `/device/{id}/key` | POST | `Devices().SetKey()` | Key expiry management is a SaaS concept |
| `/device/{id}/name` | POST | `Devices().SetName()` | headscale rename exists via gRPC but headtotails doesn't expose it |
| `/device/{id}/ip` | POST | `Devices().SetIPv4Address()` | Tailscale SaaS IP assignment feature |
| `/device/{id}/attributes` | GET | `Devices().GetPostureAttributes()` | Device posture — SaaS only |
| `/device/{id}/attributes/{key}` | POST | `Devices().SetPostureAttribute()` | Device posture — SaaS only |
| `/device/{id}/attributes/{key}` | DELETE | `Devices().DeletePostureAttribute()` | Device posture — SaaS only |

#### **DNS** (8 endpoints, 0 headscale backing)

| Endpoint | Method | Client method |
|---|---|---|
| `/tailnet/{t}/dns/nameservers` | GET | `DNS().Nameservers()` |
| `/tailnet/{t}/dns/nameservers` | POST | `DNS().SetNameservers()` |
| `/tailnet/{t}/dns/searchpaths` | GET | `DNS().SearchPaths()` |
| `/tailnet/{t}/dns/searchpaths` | POST | `DNS().SetSearchPaths()` |
| `/tailnet/{t}/dns/preferences` | GET | `DNS().Preferences()` |
| `/tailnet/{t}/dns/preferences` | POST | `DNS().SetPreferences()` |
| `/tailnet/{t}/dns/split-dns` | GET | `DNS().SplitDNS()` |
| `/tailnet/{t}/dns/split-dns` | PATCH/PUT | `DNS().UpdateSplitDNS()` / `SetSplitDNS()` |

> headscale manages DNS via its own config file, not through a REST API. No gRPC backing.

#### **Tailnet-level SaaS features** (no headscale concept)

| Endpoint | Method | Client method | Feature |
|---|---|---|---|
| `/tailnet/{t}/settings` | GET/PATCH | `TailnetSettings().Get()` / `.Update()` | Auto-updates, device approval, billing, HTTPS, regional routing |
| `/tailnet/{t}/contacts` | GET | `Contacts().Get()` | Billing/security contacts |
| `/tailnet/{t}/contacts/{type}` | PATCH | `Contacts().Update()` | Update contacts |
| `/tailnet/{t}/lock/status` | GET | *(not in Go client)* | Tailnet lock status |
| `/tailnet/{t}/vip-services` | GET/PUT/DELETE | `VIPServices().*` | Virtual IP services |

#### **Webhooks** (5 endpoints — SaaS event delivery infra)

| Endpoint | Method | Client method |
|---|---|---|
| `/tailnet/{t}/webhooks` | GET/POST | `Webhooks().List()` / `.Create()` |
| `/webhooks/{id}` | GET/PATCH/DELETE | `Webhooks().Get()` / `.Update()` / `.Delete()` |
| `/webhooks/{id}/test` | POST | `Webhooks().Test()` |
| `/webhooks/{id}/rotate` | POST | `Webhooks().RotateSecret()` |

#### **Logging** (SaaS log streaming to third-party SIEM)

| Endpoint | Method | Client method |
|---|---|---|
| `/tailnet/{t}/logging/{type}/stream` | GET/PUT/DELETE | `Logging().LogstreamConfiguration()` etc. |
| `/tailnet/{t}/logging/network` | GET | `Logging().GetNetworkFlowLogs()` |
| `/tailnet/{t}/aws-external-id` | POST | `Logging().CreateOrGetAwsExternalId()` |

#### **Posture integrations** (SaaS third-party integrations)

| Endpoint | Method | Client method |
|---|---|---|
| `/tailnet/{t}/posture/integrations` | GET/POST | `DevicePosture().ListIntegrations()` / `.CreateIntegration()` |
| `/posture/integrations/{id}` | GET/PATCH/DELETE | `DevicePosture().GetIntegration()` etc. |

#### **Keys — OAuth client & federated identity management** (SaaS-only key types)

| Endpoint | Method | Client method |
|---|---|---|
| `/tailnet/{t}/keys` (keyType=client) | POST | `Keys().CreateOAuthClient()` |
| `/tailnet/{t}/keys/{id}` (keyType=client) | PUT | `Keys().SetOAuthClient()` |
| `/tailnet/{t}/keys` (keyType=federated) | POST | `Keys().CreateFederatedIdentity()` |
| `/tailnet/{t}/keys/{id}` (keyType=federated) | PUT | `Keys().SetFederatedIdentity()` |

#### **ACL sub-endpoints**

| Endpoint | Method | Client method | Notes |
|---|---|---|---|
| `/tailnet/{t}/acl/validate` | POST | `PolicyFile().Validate()` | headscale has no validation endpoint |
| `/tailnet/{t}/acl/preview` | POST | *(not in Go client)* | SaaS policy preview |

#### **Users — single user**

| Endpoint | Method | Client method | Notes |
|---|---|---|---|
| `/users/{id}` | GET | `Users().Get()` | headtotails lists users but doesn't implement get-by-ID |

---

### Summary by the numbers

| Category | Total API endpoints | headtotails implements | SaaS-only |
|---|---|---|---|
| **OAuth** | 1 | 1 | 0 |
| **Auth keys** (basic) | 4 | 4 | 0 |
| **Keys** (OAuth/federated) | 4 | 0 | 4 |
| **Devices** (list/get/delete) | 3 | 3 | 0 |
| **Devices** (mutations) | 10 | 0 | 10 |
| **Users** | 2 | 1 (list) | 1 (get by ID) |
| **ACL** | 4 | 2 | 2 |
| **DNS** | 8 | 0 | 8 |
| **Tailnet settings** | 2 | 0 | 2 |
| **Contacts** | 2 | 0 | 2 |
| **Webhooks** | 6 | 0 | 6 |
| **Logging** | 5 | 0 | 5 |
| **Posture integrations** | 4 | 0 | 4 |
| **VIP services** | 3 | 0 | 3 |
| **Tailnet lock** | 1 | 0 | 1 |
| **Totals** | **~59** | **~11** | **~48** |

About **80% of the Tailscale API surface is SaaS-only** with no headscale backing. But critically, the **~11 endpoints headtotails implements are the ones the Tailscale Kubernetes operator actually uses** in its core flow (OAuth → create key → list devices → delete key), which is your primary use case.

The device mutation sub-resources (`/device/{id}/routes`, `/authorized`, `/tags`, etc.) are the most interesting gap — headscale *does* have partial gRPC support for some of these (routes, rename), so they represent potential future headtotails features if the operator or Terraform provider start using them.
