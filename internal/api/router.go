package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/alam0rt/headtotails/internal/headscale"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Router holds all dependencies needed to build the chi router.
type Router struct {
	hs              headscale.HeadscaleClient
	tailnetName     string
	tokenStore      *tokenStore
	headscaleAPIKey string
	clientID        string
	clientSecret    string
	hmacSecret      string
}

// NewRouter constructs a new Router.
func NewRouter(
	hs headscale.HeadscaleClient,
	tailnetName string,
	headscaleAPIKey string,
	clientID, clientSecret, hmacSecret string,
) *Router {
	return &Router{
		hs:              hs,
		tailnetName:     tailnetName,
		headscaleAPIKey: headscaleAPIKey,
		tokenStore:      newTokenStore(),
		clientID:        clientID,
		clientSecret:    clientSecret,
		hmacSecret:      hmacSecret,
	}
}

// Build constructs and returns the chi.Router.
func (ro *Router) Build() chi.Router {
	r := chi.NewRouter()

	// Core middleware.
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(prometheusMiddleware)
	r.Use(slogMiddleware)
	r.Use(middleware.Recoverer)

	// OAuth token endpoint — no auth required.
	// Registered at both paths: the legacy /oauth/token and the canonical
	// /api/v2/oauth/token that the official tailscale client library expects.
	oauthHandler := OAuthTokenHandler(ro.clientID, ro.clientSecret, ro.hmacSecret, ro.tokenStore)
	r.Post("/oauth/token", oauthHandler)
	r.Post("/api/v2/oauth/token", oauthHandler)

	// Health check — no auth required.
	r.Get("/healthz", healthzHandler)

	// Prometheus metrics — no auth required.
	r.Handle("/metrics", MetricsHandler())

	// All /api/v2 routes require authentication.
	r.Route("/api/v2", func(r chi.Router) {
		r.Use(BearerAuthMiddleware(ro.tokenStore, ro.headscaleAPIKey))

		// Devices.
		d := &devicesHandler{hs: ro.hs, tailnetName: ro.tailnetName}
		r.Get("/tailnet/{tailnet}/devices", d.ListDevices)
		r.Get("/device/{deviceId}", d.GetDevice)
		r.Delete("/device/{deviceId}", d.DeleteDevice)
		r.Post("/device/{deviceId}/authorized", d.AuthorizeDevice)
		r.Post("/device/{deviceId}/expire", d.ExpireDevice)
		r.Post("/device/{deviceId}/name", d.RenameDevice)
		r.Post("/device/{deviceId}/tags", d.SetDeviceTags)
		r.Get("/device/{deviceId}/routes", d.GetDeviceRoutes)
		r.Post("/device/{deviceId}/routes", d.SetDeviceRoutes)
		// Unimplemented device endpoints.
		deviceIP := notImplementedReason("headscale assigns IPs at registration; no runtime IP assignment API")
		r.Post("/device/{deviceId}/ip", deviceIP)
		deviceKey := notImplementedReason("device key expiry management is a Tailscale SaaS feature")
		r.Post("/device/{deviceId}/key", deviceKey)
		posture := notImplementedReason("device posture is a Tailscale SaaS feature; headscale has no posture API")
		r.Patch("/tailnet/{tailnet}/device-attributes", posture)
		r.Get("/device/{deviceId}/attributes", posture)
		r.Post("/device/{deviceId}/attributes/{key}", posture)
		r.Delete("/device/{deviceId}/attributes/{key}", posture)
		// Device invites.
		deviceInvites := notImplementedReason("device invites are a Tailscale SaaS feature; headscale has no invite concept")
		r.Get("/device/{deviceId}/device-invites", deviceInvites)
		r.Post("/device/{deviceId}/device-invites", deviceInvites)

		// Auth keys.
		k := &keysHandler{hs: ro.hs, tailnetName: ro.tailnetName}
		r.Get("/tailnet/{tailnet}/keys", k.ListKeys)
		r.Post("/tailnet/{tailnet}/keys", k.CreateKey)
		r.Get("/tailnet/{tailnet}/keys/{keyId}", k.GetKey)
		r.Delete("/tailnet/{tailnet}/keys/{keyId}", k.DeleteKey)
		r.Put("/tailnet/{tailnet}/keys/{keyId}", notImplementedReason("OAuth client and federated identity keys are Tailscale SaaS features"))

		// Users.
		u := &usersHandler{hs: ro.hs}
		r.Get("/tailnet/{tailnet}/users", u.ListUsers)
		r.Get("/users/{userId}", u.GetUser)
		r.Post("/users/{userId}/delete", u.DeleteUser)
		userSaaS := notImplementedReason("user approval/suspension/roles are Tailscale SaaS features; headscale manages users directly")
		r.Post("/users/{userId}/approve", userSaaS)
		r.Post("/users/{userId}/suspend", userSaaS)
		r.Post("/users/{userId}/restore", userSaaS)
		r.Post("/users/{userId}/role", userSaaS)

		// Policy/ACL.
		p := &policyHandler{hs: ro.hs}
		r.Get("/tailnet/{tailnet}/acl", p.GetPolicy)
		r.Post("/tailnet/{tailnet}/acl", p.SetPolicy)
		aclSaaS := notImplementedReason("headscale has no dedicated policy validation or preview gRPC endpoint")
		r.Post("/tailnet/{tailnet}/acl/preview", aclSaaS)
		r.Post("/tailnet/{tailnet}/acl/validate", aclSaaS)

		// DNS — headscale manages DNS via config file, not a runtime API.
		dns := notImplementedReason("headscale manages DNS via its config file; no gRPC API for runtime DNS changes")
		r.Get("/tailnet/{tailnet}/dns/nameservers", dns)
		r.Post("/tailnet/{tailnet}/dns/nameservers", dns)
		r.Get("/tailnet/{tailnet}/dns/preferences", dns)
		r.Post("/tailnet/{tailnet}/dns/preferences", dns)
		r.Get("/tailnet/{tailnet}/dns/searchpaths", dns)
		r.Post("/tailnet/{tailnet}/dns/searchpaths", dns)
		r.Get("/tailnet/{tailnet}/dns/configuration", dns)
		r.Post("/tailnet/{tailnet}/dns/configuration", dns)
		r.Get("/tailnet/{tailnet}/dns/split-dns", dns)
		r.Patch("/tailnet/{tailnet}/dns/split-dns", dns)
		r.Put("/tailnet/{tailnet}/dns/split-dns", dns)

		// Webhooks — headscale has no event bus or webhook dispatch.
		webhooks := notImplementedReason("webhooks are a Tailscale SaaS feature; headscale has no event bus")
		r.Get("/tailnet/{tailnet}/webhooks", webhooks)
		r.Post("/tailnet/{tailnet}/webhooks", webhooks)
		r.Get("/tailnet/{tailnet}/webhooks/{endpointId}", webhooks)
		r.Patch("/tailnet/{tailnet}/webhooks/{endpointId}", webhooks)
		r.Delete("/tailnet/{tailnet}/webhooks/{endpointId}", webhooks)
		r.Post("/tailnet/{tailnet}/webhooks/{endpointId}/rotate", webhooks)
		r.Post("/tailnet/{tailnet}/webhooks/{endpointId}/test", webhooks)

		// Logging — SaaS log streaming to third-party SIEM.
		logging := notImplementedReason("log streaming is a Tailscale SaaS feature; headscale logs to stdout")
		r.Get("/tailnet/{tailnet}/logging/{logType}", logging)
		r.Post("/tailnet/{tailnet}/logging/{logType}", logging)
		r.Get("/tailnet/{tailnet}/logging/{logType}/stream", logging)

		// Contacts — SaaS billing/security contacts.
		contacts := notImplementedReason("contacts are a Tailscale SaaS feature; headscale has no contact management")
		r.Get("/tailnet/{tailnet}/contacts", contacts)
		r.Patch("/tailnet/{tailnet}/contacts/{contactType}", contacts)

		// User invites — SaaS invite flow.
		userInvites := notImplementedReason("user invites are a Tailscale SaaS feature; headscale creates users directly")
		r.Get("/tailnet/{tailnet}/user-invites", userInvites)
		r.Post("/tailnet/{tailnet}/user-invites", userInvites)
		r.Get("/tailnet/{tailnet}/user-invites/{userInviteId}", userInvites)
		r.Delete("/tailnet/{tailnet}/user-invites/{userInviteId}", userInvites)
		r.Post("/tailnet/{tailnet}/user-invites/{userInviteId}/resend", userInvites)

		// Device posture integrations — SaaS third-party integrations.
		postureInteg := notImplementedReason("posture integrations are a Tailscale SaaS feature; headscale has no posture API")
		r.Get("/tailnet/{tailnet}/posture/integrations", postureInteg)
		r.Post("/tailnet/{tailnet}/posture/integrations", postureInteg)
		r.Get("/tailnet/{tailnet}/posture/integrations/{integrationId}", postureInteg)
		r.Patch("/tailnet/{tailnet}/posture/integrations/{integrationId}", postureInteg)
		r.Delete("/tailnet/{tailnet}/posture/integrations/{integrationId}", postureInteg)

		// Services (VIP) — SaaS networking feature.
		services := notImplementedReason("VIP services are a Tailscale SaaS feature; headscale has no equivalent")
		r.Get("/tailnet/{tailnet}/services", services)
		r.Put("/tailnet/{tailnet}/services/{serviceId}", services)
		r.Delete("/tailnet/{tailnet}/services/{serviceId}", services)

		// Tailnet settings — SaaS infrastructure.
		settings := notImplementedReason("tailnet settings (auto-updates, billing, HTTPS) are Tailscale SaaS features")
		r.Get("/tailnet/{tailnet}/settings", settings)
		r.Patch("/tailnet/{tailnet}/settings", settings)

		// Catch-all: any unmatched /api/v2 path returns 501 instead of 404.
		r.NotFound(notImplementedReason("endpoint not supported by headtotails; see https://github.com/alam0rt/headtotails for API coverage"))
	})

	return r
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func slogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}

		slog.Info("request",
			"method", r.Method,
			"route", requestRoute(r),
			"http_status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"bytes_written", ww.BytesWritten(),
			"remote_ip", requestRemoteIP(r),
			"user_agent", r.UserAgent(),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}
