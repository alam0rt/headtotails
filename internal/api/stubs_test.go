package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alam0rt/headtotails/internal/headscale"
	"github.com/alam0rt/headtotails/internal/model"
)

// TestStubEndpoints verifies that all stub (501) endpoints return the correct
// status code and a reason-specific error message.
func TestStubEndpoints(t *testing.T) {
	stubs := []struct {
		method  string
		path    string
		reason  string
	}{
		// DNS
		{http.MethodGet, "/api/v2/tailnet/-/dns/nameservers", "headscale manages DNS via its config file; no gRPC API for runtime DNS changes"},
		{http.MethodPost, "/api/v2/tailnet/-/dns/nameservers", "headscale manages DNS via its config file; no gRPC API for runtime DNS changes"},
		{http.MethodGet, "/api/v2/tailnet/-/dns/preferences", "headscale manages DNS via its config file; no gRPC API for runtime DNS changes"},
		{http.MethodPost, "/api/v2/tailnet/-/dns/preferences", "headscale manages DNS via its config file; no gRPC API for runtime DNS changes"},
		{http.MethodGet, "/api/v2/tailnet/-/dns/searchpaths", "headscale manages DNS via its config file; no gRPC API for runtime DNS changes"},
		{http.MethodPost, "/api/v2/tailnet/-/dns/searchpaths", "headscale manages DNS via its config file; no gRPC API for runtime DNS changes"},
		{http.MethodGet, "/api/v2/tailnet/-/dns/configuration", "headscale manages DNS via its config file; no gRPC API for runtime DNS changes"},
		{http.MethodPost, "/api/v2/tailnet/-/dns/configuration", "headscale manages DNS via its config file; no gRPC API for runtime DNS changes"},
		{http.MethodGet, "/api/v2/tailnet/-/dns/split-dns", "headscale manages DNS via its config file; no gRPC API for runtime DNS changes"},
		{http.MethodPatch, "/api/v2/tailnet/-/dns/split-dns", "headscale manages DNS via its config file; no gRPC API for runtime DNS changes"},
		{http.MethodPut, "/api/v2/tailnet/-/dns/split-dns", "headscale manages DNS via its config file; no gRPC API for runtime DNS changes"},
		// Webhooks
		{http.MethodGet, "/api/v2/tailnet/-/webhooks", "webhooks are a Tailscale SaaS feature; headscale has no event bus"},
		{http.MethodPost, "/api/v2/tailnet/-/webhooks", "webhooks are a Tailscale SaaS feature; headscale has no event bus"},
		// Logging
		{http.MethodGet, "/api/v2/tailnet/-/logging/configuration", "log streaming is a Tailscale SaaS feature; headscale logs to stdout"},
		{http.MethodPost, "/api/v2/tailnet/-/logging/configuration", "log streaming is a Tailscale SaaS feature; headscale logs to stdout"},
		// Contacts
		{http.MethodGet, "/api/v2/tailnet/-/contacts", "contacts are a Tailscale SaaS feature; headscale has no contact management"},
		// Posture
		{http.MethodGet, "/api/v2/tailnet/-/posture/integrations", "posture integrations are a Tailscale SaaS feature; headscale has no posture API"},
		{http.MethodPost, "/api/v2/tailnet/-/posture/integrations", "posture integrations are a Tailscale SaaS feature; headscale has no posture API"},
		// Services
		{http.MethodGet, "/api/v2/tailnet/-/services", "VIP services are a Tailscale SaaS feature; headscale has no equivalent"},
		// Tailnet settings
		{http.MethodGet, "/api/v2/tailnet/-/settings", "tailnet settings (auto-updates, billing, HTTPS) are Tailscale SaaS features"},
		{http.MethodPatch, "/api/v2/tailnet/-/settings", "tailnet settings (auto-updates, billing, HTTPS) are Tailscale SaaS features"},
		// Device stubs
		{http.MethodPost, "/api/v2/device/1/ip", "headscale assigns IPs at registration; no runtime IP assignment API"},
		{http.MethodPost, "/api/v2/device/1/key", "device key expiry management is a Tailscale SaaS feature"},
		{http.MethodGet, "/api/v2/device/1/attributes", "device posture is a Tailscale SaaS feature; headscale has no posture API"},
	}

	m := &headscale.MockHeadscaleClient{}
	router, tok := setupTestRouter(m)

	for _, s := range stubs {
		t.Run(s.method+" "+s.path, func(t *testing.T) {
			req := httptest.NewRequest(s.method, s.path, nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotImplemented, w.Code,
				"expected 501 for %s %s", s.method, s.path)
			var errResp model.Error
			require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
			assert.Equal(t, s.reason, errResp.Message)
		})
	}
}

// TestCatchAllReturns501 verifies that unknown /api/v2 paths return 501
// instead of 404, so clients get a clear "not supported" signal.
func TestCatchAllReturns501(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	router, tok := setupTestRouter(m)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/tailnet/-/some-future-endpoint", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
	var errResp model.Error
	require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
	assert.Contains(t, errResp.Message, "not supported by headtotails")
}

func TestHealthz(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	router, _ := setupTestRouter(m)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "ok", resp["status"])
}
