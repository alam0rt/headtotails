package api

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test JWT helpers ---

// testJWTKey holds a key pair and kid for test JWT signing.
type testJWTKey struct {
	kid        string
	privateKey *ecdsa.PrivateKey
}

func newTestJWTKey(t *testing.T) *testJWTKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return &testJWTKey{kid: "test-kid-1", privateKey: priv}
}

// signJWT creates a signed ES256 JWT with the given claims.
func (k *testJWTKey) signJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := map[string]string{
		"alg": "ES256",
		"typ": "JWT",
		"kid": k.kid,
	}
	headerJSON, err := json.Marshal(header)
	require.NoError(t, err)
	claimsJSON, err := json.Marshal(claims)
	require.NoError(t, err)

	h64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	c64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signed := h64 + "." + c64

	hash := sha256.Sum256([]byte(signed))
	sig, err := ecdsa.SignASN1(rand.Reader, k.privateKey, hash[:])
	require.NoError(t, err)

	s64 := base64.RawURLEncoding.EncodeToString(sig)
	return signed + "." + s64
}

// jwkJSON returns the JWK JSON for the public key.
func (k *testJWTKey) jwkJSON(t *testing.T) json.RawMessage {
	t.Helper()
	pub := k.privateKey.PublicKey
	jwk := map[string]string{
		"kty": "EC",
		"crv": "P-256",
		"kid": k.kid,
		"x":   base64.RawURLEncoding.EncodeToString(pub.X.Bytes()),
		"y":   base64.RawURLEncoding.EncodeToString(pub.Y.Bytes()),
		"use": "sig",
		"alg": "ES256",
	}
	b, err := json.Marshal(jwk)
	require.NoError(t, err)
	return b
}

// testRSAKey holds an RSA key pair for test JWT signing.
type testRSAKey struct {
	kid        string
	privateKey *rsa.PrivateKey
}

func newTestRSAKey(t *testing.T) *testRSAKey {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return &testRSAKey{kid: "test-rsa-kid-1", privateKey: priv}
}

func (k *testRSAKey) signJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
		"kid": k.kid,
	}
	headerJSON, err := json.Marshal(header)
	require.NoError(t, err)
	claimsJSON, err := json.Marshal(claims)
	require.NoError(t, err)

	h64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	c64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signed := h64 + "." + c64

	hash := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, k.privateKey, crypto.SHA256, hash[:])
	require.NoError(t, err)

	s64 := base64.RawURLEncoding.EncodeToString(sig)
	return signed + "." + s64
}

func (k *testRSAKey) jwkJSON(t *testing.T) json.RawMessage {
	t.Helper()
	pub := k.privateKey.PublicKey
	eBytes := big.NewInt(int64(pub.E)).Bytes()
	jwk := map[string]string{
		"kty": "RSA",
		"kid": k.kid,
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(eBytes),
		"use": "sig",
		"alg": "RS256",
	}
	b, err := json.Marshal(jwk)
	require.NoError(t, err)
	return b
}

// --- Fake OIDC discovery + JWKS server ---

func startFakeOIDCServer(t *testing.T, jwkKeys ...json.RawMessage) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		// Dynamically build the issuer URL from the request.
		// Since tests replace the issuer after server creation, we can't hard-code it.
		w.Header().Set("Content-Type", "application/json")
		scheme := "http"
		issuer := fmt.Sprintf("%s://%s", scheme, r.Host)
		json.NewEncoder(w).Encode(map[string]string{
			"issuer":   issuer,
			"jwks_uri": issuer + "/.well-known/jwks.json",
		})
	})
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"keys": jwkKeys,
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// --- mockJWTValidator for unit-testing TokenExchangeHandler in isolation ---

type mockJWTValidator struct {
	claims *JWTClaims
	err    error
}

func (m *mockJWTValidator) Validate(_ context.Context, _ string) (*JWTClaims, error) {
	return m.claims, m.err
}

// === Tests ===

func TestTokenExchangeHandler_ValidJWT(t *testing.T) {
	store := newTokenStore()
	validator := &mockJWTValidator{
		claims: &JWTClaims{
			Issuer:    "https://kubernetes.default.svc",
			Subject:   "system:serviceaccount:tailscale:operator",
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		},
	}
	handler := TokenExchangeHandler(validator, testHMACSecret, "wif-client-id", store)

	body := url.Values{
		"grant_type": {"client_credentials"},
		"jwt":        {"fake-jwt-token"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v2/oauth/token-exchange",
		strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("wif-client-id", "")
	w := httptest.NewRecorder()

	handler(w, req)

	require.Equal(t, http.StatusOK, w.Code, "valid JWT exchange should return 200")
	var resp oauthTokenResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotEmpty(t, resp.AccessToken)
	assert.Equal(t, "Bearer", resp.TokenType)
	assert.Greater(t, resp.ExpiresIn, 0)

	// Token should be usable with BearerAuthMiddleware.
	entry, ok := store.lookup(resp.AccessToken)
	assert.True(t, ok, "token should be stored")
	assert.True(t, entry.expiry.After(time.Now()), "token should not be expired")
}

func TestTokenExchangeHandler_ClientIDFromFormBody(t *testing.T) {
	store := newTokenStore()
	validator := &mockJWTValidator{
		claims: &JWTClaims{
			Issuer:    "https://kubernetes.default.svc",
			Subject:   "system:serviceaccount:tailscale:operator",
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		},
	}
	handler := TokenExchangeHandler(validator, testHMACSecret, "wif-client-id", store)

	body := url.Values{
		"grant_type": {"client_credentials"},
		"client_id":  {"wif-client-id"},
		"jwt":        {"fake-jwt-token"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v2/oauth/token-exchange",
		strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler(w, req)

	require.Equal(t, http.StatusOK, w.Code, "client_id from form body should work")
}

func TestTokenExchangeHandler_InvalidJWT(t *testing.T) {
	store := newTokenStore()
	validator := &mockJWTValidator{
		err: fmt.Errorf("wif: signature verification failed"),
	}
	handler := TokenExchangeHandler(validator, testHMACSecret, "wif-client-id", store)

	body := url.Values{
		"grant_type": {"client_credentials"},
		"jwt":        {"invalid-jwt"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v2/oauth/token-exchange",
		strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("wif-client-id", "")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "invalid JWT should be rejected")
}

func TestTokenExchangeHandler_MissingJWT(t *testing.T) {
	store := newTokenStore()
	validator := &mockJWTValidator{}
	handler := TokenExchangeHandler(validator, testHMACSecret, "wif-client-id", store)

	body := url.Values{
		"grant_type": {"client_credentials"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v2/oauth/token-exchange",
		strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("wif-client-id", "")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "missing jwt parameter should return 400")
}

func TestTokenExchangeHandler_MissingClientID(t *testing.T) {
	store := newTokenStore()
	validator := &mockJWTValidator{}
	handler := TokenExchangeHandler(validator, testHMACSecret, "wif-client-id", store)

	body := url.Values{
		"grant_type": {"client_credentials"},
		"jwt":        {"some-jwt"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v2/oauth/token-exchange",
		strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "missing client_id should return 400")
}

func TestTokenExchangeHandler_WrongClientID(t *testing.T) {
	store := newTokenStore()
	validator := &mockJWTValidator{}
	handler := TokenExchangeHandler(validator, testHMACSecret, "wif-client-id", store)

	body := url.Values{
		"grant_type": {"client_credentials"},
		"jwt":        {"some-jwt"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v2/oauth/token-exchange",
		strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("wrong-client-id", "")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "wrong client_id should return 401")
}

func TestTokenExchangeHandler_WrongGrantType(t *testing.T) {
	store := newTokenStore()
	validator := &mockJWTValidator{}
	handler := TokenExchangeHandler(validator, testHMACSecret, "wif-client-id", store)

	body := url.Values{
		"grant_type": {"authorization_code"},
		"jwt":        {"some-jwt"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v2/oauth/token-exchange",
		strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("wif-client-id", "")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "wrong grant_type should return 400")
}

func TestTokenExchangeHandler_NilValidator(t *testing.T) {
	store := newTokenStore()
	handler := TokenExchangeHandler(nil, testHMACSecret, "wif-client-id", store)

	body := url.Values{
		"grant_type": {"client_credentials"},
		"jwt":        {"some-jwt"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v2/oauth/token-exchange",
		strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("wif-client-id", "")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code,
		"nil validator (WIF not configured) should return 501")
}

// --- OIDCValidator integration tests using a real ECDSA key + fake OIDC server ---

func TestOIDCValidator_ValidES256JWT(t *testing.T) {
	key := newTestJWTKey(t)
	srv := startFakeOIDCServer(t, key.jwkJSON(t))

	validator := NewOIDCValidator(srv.URL, "", "test-client", srv.Client())

	jwt := key.signJWT(t, map[string]any{
		"iss": srv.URL,
		"sub": "system:serviceaccount:tailscale:operator",
		"aud": "test-client",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})

	claims, err := validator.Validate(context.Background(), jwt)
	require.NoError(t, err)
	assert.Equal(t, srv.URL, claims.Issuer)
	assert.Equal(t, "system:serviceaccount:tailscale:operator", claims.Subject)
}

func TestOIDCValidator_ValidRS256JWT(t *testing.T) {
	rsaKey := newTestRSAKey(t)
	srv := startFakeOIDCServer(t, rsaKey.jwkJSON(t))

	validator := NewOIDCValidator(srv.URL, "", "test-client", srv.Client())

	jwt := rsaKey.signJWT(t, map[string]any{
		"iss": srv.URL,
		"sub": "system:serviceaccount:tailscale:operator",
		"aud": "test-client",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})

	claims, err := validator.Validate(context.Background(), jwt)
	require.NoError(t, err)
	assert.Equal(t, srv.URL, claims.Issuer)
	assert.Equal(t, "system:serviceaccount:tailscale:operator", claims.Subject)
}

func TestOIDCValidator_ExpiredJWT(t *testing.T) {
	key := newTestJWTKey(t)
	srv := startFakeOIDCServer(t, key.jwkJSON(t))

	validator := NewOIDCValidator(srv.URL, "", "test-client", srv.Client())

	jwt := key.signJWT(t, map[string]any{
		"iss": srv.URL,
		"sub": "system:serviceaccount:tailscale:operator",
		"exp": time.Now().Add(-time.Hour).Unix(), // expired
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	})

	_, err := validator.Validate(context.Background(), jwt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestOIDCValidator_WrongIssuer(t *testing.T) {
	key := newTestJWTKey(t)
	srv := startFakeOIDCServer(t, key.jwkJSON(t))

	validator := NewOIDCValidator(srv.URL, "", "test-client", srv.Client())

	jwt := key.signJWT(t, map[string]any{
		"iss": "https://wrong-issuer.example.com",
		"sub": "system:serviceaccount:tailscale:operator",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err := validator.Validate(context.Background(), jwt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "issuer mismatch")
}

func TestOIDCValidator_WrongAudience(t *testing.T) {
	key := newTestJWTKey(t)
	srv := startFakeOIDCServer(t, key.jwkJSON(t))

	// Validator expects audience "expected-audience"
	validator := NewOIDCValidator(srv.URL, "expected-audience", "test-client", srv.Client())

	jwt := key.signJWT(t, map[string]any{
		"iss": srv.URL,
		"sub": "system:serviceaccount:tailscale:operator",
		"aud": "wrong-audience",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err := validator.Validate(context.Background(), jwt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "audience")
}

func TestOIDCValidator_AudienceNotRequiredWhenEmpty(t *testing.T) {
	key := newTestJWTKey(t)
	srv := startFakeOIDCServer(t, key.jwkJSON(t))

	// Empty audience means no audience check.
	validator := NewOIDCValidator(srv.URL, "", "test-client", srv.Client())

	jwt := key.signJWT(t, map[string]any{
		"iss": srv.URL,
		"sub": "system:serviceaccount:tailscale:operator",
		"aud": "anything",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	claims, err := validator.Validate(context.Background(), jwt)
	require.NoError(t, err)
	assert.Equal(t, "system:serviceaccount:tailscale:operator", claims.Subject)
}

func TestOIDCValidator_AudienceArrayClaim(t *testing.T) {
	key := newTestJWTKey(t)
	srv := startFakeOIDCServer(t, key.jwkJSON(t))

	validator := NewOIDCValidator(srv.URL, "expected-audience", "test-client", srv.Client())

	jwt := key.signJWT(t, map[string]any{
		"iss": srv.URL,
		"sub": "test-sub",
		"aud": []string{"other-aud", "expected-audience"},
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	claims, err := validator.Validate(context.Background(), jwt)
	require.NoError(t, err)
	assert.Equal(t, "test-sub", claims.Subject)
}

func TestOIDCValidator_UnknownKid(t *testing.T) {
	// Create one key for signing but serve a different key's JWK.
	signingKey := newTestJWTKey(t)
	otherKey := newTestJWTKey(t)
	otherKey.kid = "different-kid"

	srv := startFakeOIDCServer(t, otherKey.jwkJSON(t))
	validator := NewOIDCValidator(srv.URL, "", "test-client", srv.Client())

	jwt := signingKey.signJWT(t, map[string]any{
		"iss": srv.URL,
		"sub": "test-sub",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err := validator.Validate(context.Background(), jwt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kid")
}

func TestOIDCValidator_TamperedJWT(t *testing.T) {
	key := newTestJWTKey(t)
	srv := startFakeOIDCServer(t, key.jwkJSON(t))

	validator := NewOIDCValidator(srv.URL, "", "test-client", srv.Client())

	jwt := key.signJWT(t, map[string]any{
		"iss": srv.URL,
		"sub": "original-subject",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	// Tamper with the claims (replace subject).
	parts := strings.SplitN(jwt, ".", 3)
	tampered := map[string]any{
		"iss": srv.URL,
		"sub": "evil-subject",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	tamperedJSON, _ := json.Marshal(tampered)
	parts[1] = base64.RawURLEncoding.EncodeToString(tamperedJSON)
	tamperedJWT := strings.Join(parts, ".")

	_, err := validator.Validate(context.Background(), tamperedJWT)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signature verification failed")
}

func TestOIDCValidator_MalformedJWT(t *testing.T) {
	key := newTestJWTKey(t)
	srv := startFakeOIDCServer(t, key.jwkJSON(t))
	validator := NewOIDCValidator(srv.URL, "", "test-client", srv.Client())

	tests := []struct {
		name string
		jwt  string
	}{
		{"empty", ""},
		{"one part", "header"},
		{"two parts", "header.payload"},
		{"bad header base64", "!!!.payload.sig"},
		{"bad header json", base64.RawURLEncoding.EncodeToString([]byte("not-json")) + ".payload.sig"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validator.Validate(context.Background(), tt.jwt)
			require.Error(t, err)
		})
	}
}

func TestOIDCValidator_JWKSCaching(t *testing.T) {
	key := newTestJWTKey(t)
	fetchCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		issuer := fmt.Sprintf("http://%s", r.Host)
		json.NewEncoder(w).Encode(map[string]string{
			"issuer":   issuer,
			"jwks_uri": issuer + "/.well-known/jwks.json",
		})
	})
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"keys": []json.RawMessage{key.jwkJSON(t)},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	validator := NewOIDCValidator(srv.URL, "", "test-client", srv.Client())

	claims := map[string]any{
		"iss": srv.URL,
		"sub": "test-sub",
		"exp": time.Now().Add(time.Hour).Unix(),
	}

	// First call fetches JWKS.
	jwt1 := key.signJWT(t, claims)
	_, err := validator.Validate(context.Background(), jwt1)
	require.NoError(t, err)
	assert.Equal(t, 1, fetchCount, "first call should fetch JWKS")

	// Second call should use cache.
	jwt2 := key.signJWT(t, claims)
	_, err = validator.Validate(context.Background(), jwt2)
	require.NoError(t, err)
	assert.Equal(t, 1, fetchCount, "second call should use cached JWKS")
}

func TestOIDCValidator_JWKSDiscoveryFailure(t *testing.T) {
	// Point at a server that returns errors.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	key := newTestJWTKey(t)
	validator := NewOIDCValidator(srv.URL, "", "test-client", srv.Client())

	jwt := key.signJWT(t, map[string]any{
		"iss": srv.URL,
		"sub": "test-sub",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err := validator.Validate(context.Background(), jwt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OIDC discovery")
}

// --- End-to-end test: TokenExchangeHandler + OIDCValidator ---

func TestTokenExchangeHandler_EndToEnd_ES256(t *testing.T) {
	key := newTestJWTKey(t)
	srv := startFakeOIDCServer(t, key.jwkJSON(t))

	store := newTokenStore()
	validator := NewOIDCValidator(srv.URL, "", "wif-client-id", srv.Client())
	handler := TokenExchangeHandler(validator, testHMACSecret, "wif-client-id", store)

	jwt := key.signJWT(t, map[string]any{
		"iss": srv.URL,
		"sub": "system:serviceaccount:tailscale:operator",
		"aud": "wif-client-id",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
	})

	// Mimic exactly what the k8s-operator does:
	// Basic Auth with client_id as username, empty password.
	body := url.Values{
		"grant_type": {"client_credentials"},
		"jwt":        {jwt},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v2/oauth/token-exchange",
		strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("wif-client-id", "")
	w := httptest.NewRecorder()

	handler(w, req)

	require.Equal(t, http.StatusOK, w.Code,
		"end-to-end WIF token exchange should succeed, body: %s", w.Body.String())

	var resp oauthTokenResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotEmpty(t, resp.AccessToken)
	assert.Equal(t, "Bearer", resp.TokenType)

	// The issued token should be usable with BearerAuthMiddleware.
	mw := BearerAuthMiddleware(store, "")
	protected := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	apiReq := httptest.NewRequest(http.MethodGet, "/api/v2/tailnet/-/keys", nil)
	apiReq.Header.Set("Authorization", "Bearer "+resp.AccessToken)
	apiW := httptest.NewRecorder()
	protected.ServeHTTP(apiW, apiReq)
	assert.Equal(t, http.StatusOK, apiW.Code,
		"WIF-issued token should pass BearerAuthMiddleware")
}

func TestTokenExchangeHandler_EndToEnd_RS256(t *testing.T) {
	rsaKey := newTestRSAKey(t)
	srv := startFakeOIDCServer(t, rsaKey.jwkJSON(t))

	store := newTokenStore()
	validator := NewOIDCValidator(srv.URL, "", "wif-client-id", srv.Client())
	handler := TokenExchangeHandler(validator, testHMACSecret, "wif-client-id", store)

	jwt := rsaKey.signJWT(t, map[string]any{
		"iss": srv.URL,
		"sub": "system:serviceaccount:tailscale:operator",
		"aud": "wif-client-id",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
	})

	body := url.Values{
		"grant_type": {"client_credentials"},
		"jwt":        {jwt},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v2/oauth/token-exchange",
		strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("wif-client-id", "")
	w := httptest.NewRecorder()

	handler(w, req)

	require.Equal(t, http.StatusOK, w.Code,
		"end-to-end RS256 WIF token exchange should succeed, body: %s", w.Body.String())
}

// --- JWTClaims.AudienceContains ---

func TestJWTClaims_AudienceContains(t *testing.T) {
	tests := []struct {
		name     string
		audience any
		check    string
		want     bool
	}{
		{"string match", "aud1", "aud1", true},
		{"string no match", "aud1", "aud2", false},
		{"array match", []any{"aud1", "aud2"}, "aud2", true},
		{"array no match", []any{"aud1", "aud2"}, "aud3", false},
		{"nil audience", nil, "aud1", false},
		{"empty string", "", "aud1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &JWTClaims{Audience: tt.audience}
			assert.Equal(t, tt.want, c.AudienceContains(tt.check))
		})
	}
}
