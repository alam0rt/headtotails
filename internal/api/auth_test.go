package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuthTokenHandler(t *testing.T) {
	const (
		clientID     = "test-client"
		clientSecret = "test-secret"
		hmacSecret   = "hmac-secret-32-chars-padding!!"
	)

	store := newTokenStore()
	handler := OAuthTokenHandler(clientID, clientSecret, hmacSecret, store)

	tests := []struct {
		name       string
		body       url.Values
		wantStatus int
		wantToken  bool
	}{
		{
			name: "valid client_credentials",
			body: url.Values{
				"grant_type":    {"client_credentials"},
				"client_id":     {clientID},
				"client_secret": {clientSecret},
			},
			wantStatus: http.StatusOK,
			wantToken:  true,
		},
		{
			name: "wrong secret",
			body: url.Values{
				"grant_type":    {"client_credentials"},
				"client_id":     {clientID},
				"client_secret": {"wrong-secret"},
			},
			wantStatus: http.StatusUnauthorized,
			wantToken:  false,
		},
		{
			name: "missing grant_type",
			body: url.Values{
				"client_id":     {clientID},
				"client_secret": {clientSecret},
			},
			wantStatus: http.StatusBadRequest,
			wantToken:  false,
		},
		{
			name: "unsupported grant_type",
			body: url.Values{
				"grant_type":    {"password"},
				"client_id":     {clientID},
				"client_secret": {clientSecret},
			},
			wantStatus: http.StatusBadRequest,
			wantToken:  false,
		},
		{
			name:       "missing credentials",
			body:       url.Values{"grant_type": {"client_credentials"}},
			wantStatus: http.StatusBadRequest,
			wantToken:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/oauth/token",
				strings.NewReader(tt.body.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			handler(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.wantToken {
				var resp oauthTokenResponse
				require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
				assert.NotEmpty(t, resp.AccessToken)
				assert.Equal(t, "Bearer", resp.TokenType)
				assert.Greater(t, resp.ExpiresIn, 0)
			} else {
				var errResp struct {
					Message string `json:"message"`
				}
				require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
				assert.NotEmpty(t, errResp.Message)
			}
		})
	}
}

func TestOAuthTokenHandlerBasicAuth(t *testing.T) {
	store := newTokenStore()
	handler := OAuthTokenHandler("myid", "mysecret", "hmac-secret", store)

	body := url.Values{"grant_type": {"client_credentials"}}
	req := httptest.NewRequest(http.MethodPost, "/oauth/token",
		strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("myid", "mysecret")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBearerAuthMiddleware(t *testing.T) {
	const headscaleKey = "hskey-abc123"

	store := newTokenStore()
	mw := BearerAuthMiddleware(store, headscaleKey)

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	protected := mw(okHandler)

	t.Run("no token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v2/device/1", nil)
		w := httptest.NewRecorder()
		protected.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("headscale API key as bearer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v2/device/1", nil)
		req.Header.Set("Authorization", "Bearer "+headscaleKey)
		w := httptest.NewRecorder()
		protected.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("headscale API key as basic auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v2/device/1", nil)
		req.SetBasicAuth(headscaleKey, "")
		w := httptest.NewRecorder()
		protected.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("valid OAuth token", func(t *testing.T) {
		tok, err := generateToken("secret")
		require.NoError(t, err)
		// Manually insert a non-expired token.
		store2 := newTokenStore()
		_ = tok // suppress unused warning
		tok2, err := generateToken("secret2")
		require.NoError(t, err)
		store2.store(tok2, tokenEntry{expiry: futureTime()})
		mw2 := BearerAuthMiddleware(store2, headscaleKey)
		protected2 := mw2(okHandler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+tok2)
		w := httptest.NewRecorder()
		protected2.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("expired OAuth token", func(t *testing.T) {
		store3 := newTokenStore()
		store3.store("expired-tok", tokenEntry{expiry: pastTime()})
		mw3 := BearerAuthMiddleware(store3, headscaleKey)
		protected3 := mw3(okHandler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer expired-tok")
		w := httptest.NewRecorder()
		protected3.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func futureTime() time.Time { return time.Now().Add(time.Hour) }
func pastTime() time.Time   { return time.Now().Add(-time.Hour) }

func TestTokenStorePurge(t *testing.T) {
	store := &tokenStore{
		tokens: map[string]tokenEntry{
			"expired": {expiry: time.Now().Add(-time.Minute)},
			"valid":   {expiry: time.Now().Add(time.Minute)},
		},
	}

	store.purge()

	_, expiredOK := store.lookup("expired")
	_, validOK := store.lookup("valid")
	assert.False(t, expiredOK)
	assert.True(t, validOK)
}

func TestParseOAuthCredentialsParseFormError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", io.NopCloser(errorReader{}))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	id, secret, ok := parseOAuthCredentials(req)
	assert.False(t, ok)
	assert.Empty(t, id)
	assert.Empty(t, secret)
}

type errorReader struct{}

func (errorReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read failure")
}
