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
// status code and error body — ensuring the contract is regression-protected.
func TestStubEndpoints(t *testing.T) {
	stubs := []struct {
		method string
		path   string
	}{
		// DNS
		{http.MethodGet, "/api/v2/tailnet/-/dns/nameservers"},
		{http.MethodPost, "/api/v2/tailnet/-/dns/nameservers"},
		{http.MethodGet, "/api/v2/tailnet/-/dns/preferences"},
		{http.MethodPost, "/api/v2/tailnet/-/dns/preferences"},
		{http.MethodGet, "/api/v2/tailnet/-/dns/searchpaths"},
		{http.MethodPost, "/api/v2/tailnet/-/dns/searchpaths"},
		{http.MethodGet, "/api/v2/tailnet/-/dns/configuration"},
		{http.MethodPost, "/api/v2/tailnet/-/dns/configuration"},
		{http.MethodGet, "/api/v2/tailnet/-/dns/split-dns"},
		{http.MethodPatch, "/api/v2/tailnet/-/dns/split-dns"},
		{http.MethodPut, "/api/v2/tailnet/-/dns/split-dns"},
		// Webhooks
		{http.MethodGet, "/api/v2/tailnet/-/webhooks"},
		{http.MethodPost, "/api/v2/tailnet/-/webhooks"},
		// Logging
		{http.MethodGet, "/api/v2/tailnet/-/logging/configuration"},
		{http.MethodPost, "/api/v2/tailnet/-/logging/configuration"},
		// Contacts
		{http.MethodGet, "/api/v2/tailnet/-/contacts"},
		// Posture
		{http.MethodGet, "/api/v2/tailnet/-/posture/integrations"},
		{http.MethodPost, "/api/v2/tailnet/-/posture/integrations"},
		// Services
		{http.MethodGet, "/api/v2/tailnet/-/services"},
		// Tailnet settings
		{http.MethodGet, "/api/v2/tailnet/-/settings"},
		{http.MethodPatch, "/api/v2/tailnet/-/settings"},
		// Device stubs
		{http.MethodPost, "/api/v2/device/1/ip"},
		{http.MethodPost, "/api/v2/device/1/key"},
		{http.MethodGet, "/api/v2/device/1/attributes"},
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
			assert.Equal(t, "not implemented by headtotails", errResp.Message)
		})
	}
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
