package api

import (
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/alam0rt/headtotails/internal/headscale"
)

const (
	testClientID     = "test-client"
	testClientSecret = "test-secret"
	testHMACSecret   = "test-hmac-secret-32chars-padded!"
	testHeadscaleKey = "test-headscale-api-key"
	testTailnet      = "-"
)

// setupTestRouter creates a chi router with the given HeadscaleClient mock,
// using a pre-seeded token store so tests can use testBearerToken directly.
func setupTestRouter(hs headscale.HeadscaleClient) (http.Handler, string) {
	store := newTokenStore()
	tok := "test-valid-token"
	store.store(tok, tokenEntry{expiry: time.Now().Add(time.Hour)})

	ro := &Router{
		hs:              hs,
		tailnetName:     testTailnet,
		headscaleAPIKey: testHeadscaleKey,
		tokenStore:      store,
		clientID:        testClientID,
		clientSecret:    testClientSecret,
		hmacSecret:      testHMACSecret,
	}
	return ro.Build(), tok
}

// doRequest performs an HTTP request against the handler and returns the recorder.
func doRequest(handler http.Handler, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}
