package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/alam0rt/headtotails/internal/headscale"
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
		r.Post("/device/{deviceId}/ip", notImplemented)
		r.Post("/device/{deviceId}/key", notImplemented)
		r.Patch("/tailnet/{tailnet}/device-attributes", notImplemented)
		r.Get("/device/{deviceId}/attributes", notImplemented)
		r.Post("/device/{deviceId}/attributes/{key}", notImplemented)
		r.Delete("/device/{deviceId}/attributes/{key}", notImplemented)
		// Device invites.
		r.Get("/device/{deviceId}/device-invites", notImplemented)
		r.Post("/device/{deviceId}/device-invites", notImplemented)

		// Auth keys.
		k := &keysHandler{hs: ro.hs, tailnetName: ro.tailnetName}
		r.Get("/tailnet/{tailnet}/keys", k.ListKeys)
		r.Post("/tailnet/{tailnet}/keys", k.CreateKey)
		r.Get("/tailnet/{tailnet}/keys/{keyId}", k.GetKey)
		r.Delete("/tailnet/{tailnet}/keys/{keyId}", k.DeleteKey)
		r.Put("/tailnet/{tailnet}/keys/{keyId}", notImplemented)

		// Users.
		u := &usersHandler{hs: ro.hs}
		r.Get("/tailnet/{tailnet}/users", u.ListUsers)
		r.Get("/users/{userId}", u.GetUser)
		r.Post("/users/{userId}/delete", u.DeleteUser)
		r.Post("/users/{userId}/approve", notImplemented)
		r.Post("/users/{userId}/suspend", notImplemented)
		r.Post("/users/{userId}/restore", notImplemented)
		r.Post("/users/{userId}/role", notImplemented)

		// Policy/ACL.
		p := &policyHandler{hs: ro.hs}
		r.Get("/tailnet/{tailnet}/acl", p.GetPolicy)
		r.Post("/tailnet/{tailnet}/acl", p.SetPolicy)
		r.Post("/tailnet/{tailnet}/acl/preview", notImplemented)
		r.Post("/tailnet/{tailnet}/acl/validate", notImplemented)

		// DNS — all stubs.
		r.Get("/tailnet/{tailnet}/dns/nameservers", notImplemented)
		r.Post("/tailnet/{tailnet}/dns/nameservers", notImplemented)
		r.Get("/tailnet/{tailnet}/dns/preferences", notImplemented)
		r.Post("/tailnet/{tailnet}/dns/preferences", notImplemented)
		r.Get("/tailnet/{tailnet}/dns/searchpaths", notImplemented)
		r.Post("/tailnet/{tailnet}/dns/searchpaths", notImplemented)
		r.Get("/tailnet/{tailnet}/dns/configuration", notImplemented)
		r.Post("/tailnet/{tailnet}/dns/configuration", notImplemented)
		r.Get("/tailnet/{tailnet}/dns/split-dns", notImplemented)
		r.Patch("/tailnet/{tailnet}/dns/split-dns", notImplemented)
		r.Put("/tailnet/{tailnet}/dns/split-dns", notImplemented)

		// Webhooks — all stubs.
		r.Get("/tailnet/{tailnet}/webhooks", notImplemented)
		r.Post("/tailnet/{tailnet}/webhooks", notImplemented)
		r.Get("/tailnet/{tailnet}/webhooks/{endpointId}", notImplemented)
		r.Patch("/tailnet/{tailnet}/webhooks/{endpointId}", notImplemented)
		r.Delete("/tailnet/{tailnet}/webhooks/{endpointId}", notImplemented)
		r.Post("/tailnet/{tailnet}/webhooks/{endpointId}/rotate", notImplemented)
		r.Post("/tailnet/{tailnet}/webhooks/{endpointId}/test", notImplemented)

		// Logging — all stubs.
		r.Get("/tailnet/{tailnet}/logging/{logType}", notImplemented)
		r.Post("/tailnet/{tailnet}/logging/{logType}", notImplemented)
		r.Get("/tailnet/{tailnet}/logging/{logType}/stream", notImplemented)

		// Contacts — all stubs.
		r.Get("/tailnet/{tailnet}/contacts", notImplemented)
		r.Patch("/tailnet/{tailnet}/contacts/{contactType}", notImplemented)

		// User invites — all stubs.
		r.Get("/tailnet/{tailnet}/user-invites", notImplemented)
		r.Post("/tailnet/{tailnet}/user-invites", notImplemented)
		r.Get("/tailnet/{tailnet}/user-invites/{userInviteId}", notImplemented)
		r.Delete("/tailnet/{tailnet}/user-invites/{userInviteId}", notImplemented)
		r.Post("/tailnet/{tailnet}/user-invites/{userInviteId}/resend", notImplemented)

		// Device posture — all stubs.
		r.Get("/tailnet/{tailnet}/posture/integrations", notImplemented)
		r.Post("/tailnet/{tailnet}/posture/integrations", notImplemented)
		r.Get("/tailnet/{tailnet}/posture/integrations/{integrationId}", notImplemented)
		r.Patch("/tailnet/{tailnet}/posture/integrations/{integrationId}", notImplemented)
		r.Delete("/tailnet/{tailnet}/posture/integrations/{integrationId}", notImplemented)

		// Services (VIP) — all stubs.
		r.Get("/tailnet/{tailnet}/services", notImplemented)
		r.Put("/tailnet/{tailnet}/services/{serviceId}", notImplemented)
		r.Delete("/tailnet/{tailnet}/services/{serviceId}", notImplemented)

		// Tailnet settings — all stubs.
		r.Get("/tailnet/{tailnet}/settings", notImplemented)
		r.Patch("/tailnet/{tailnet}/settings", notImplemented)
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
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}
