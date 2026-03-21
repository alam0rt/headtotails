package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const oauthTokenTTL = time.Hour

// tokenEntry stores metadata about an issued OAuth token.
type tokenEntry struct {
	expiry  time.Time
	scopes  []string
}

// tokenStore is an in-memory store for OAuth tokens.
type tokenStore struct {
	mu     sync.RWMutex
	tokens map[string]tokenEntry
}

func newTokenStore() *tokenStore {
	ts := &tokenStore{tokens: make(map[string]tokenEntry)}
	go ts.purgeLoop()
	return ts
}

func (ts *tokenStore) store(token string, entry tokenEntry) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.tokens[token] = entry
}

func (ts *tokenStore) lookup(token string) (tokenEntry, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	e, ok := ts.tokens[token]
	return e, ok
}

func (ts *tokenStore) purgeLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ts.purge()
	}
}

func (ts *tokenStore) purge() {
	now := time.Now()
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for tok, e := range ts.tokens {
		if now.After(e.expiry) {
			delete(ts.tokens, tok)
		}
	}
}

// oauthTokenResponse is the Tailscale OAuth token response shape.
type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// OAuthTokenHandler returns an http.HandlerFunc for POST /oauth/token.
// clientID, clientSecret are the expected credentials; hmacSecret is used
// to sign tokens; headscaleAPIKey is accepted as an additional valid token.
func OAuthTokenHandler(clientID, clientSecret, hmacSecret string, store *tokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse credentials from Basic Auth or form body.
		id, secret, ok := parseOAuthCredentials(r)
		if !ok {
			writeError(w, http.StatusBadRequest, "missing credentials")
			return
		}

		grantType := r.FormValue("grant_type")
		if grantType == "" {
			writeError(w, http.StatusBadRequest, "missing grant_type")
			return
		}
		if grantType != "client_credentials" {
			writeError(w, http.StatusBadRequest, "unsupported grant_type; only client_credentials is supported")
			return
		}

		if id != clientID || secret != clientSecret {
			writeError(w, http.StatusUnauthorized, "invalid client credentials")
			return
		}

		tok, err := generateToken(hmacSecret)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate token")
			return
		}

		expiry := time.Now().Add(oauthTokenTTL)
		store.store(tok, tokenEntry{expiry: expiry})

		writeJSON(w, http.StatusOK, oauthTokenResponse{
			AccessToken: tok,
			TokenType:   "Bearer",
			ExpiresIn:   int(oauthTokenTTL.Seconds()),
		})
	}
}

// parseOAuthCredentials extracts client_id and client_secret from Basic Auth
// or from the request form body.
func parseOAuthCredentials(r *http.Request) (id, secret string, ok bool) {
	// Try Basic Auth first.
	id, secret, ok = r.BasicAuth()
	if ok && id != "" {
		return
	}

	// Fall back to form body (requires ParseForm to have been called).
	if err := r.ParseForm(); err != nil {
		return "", "", false
	}
	id = r.FormValue("client_id")
	secret = r.FormValue("client_secret")
	ok = id != "" && secret != ""
	return
}

// generateToken creates an HMAC-signed opaque token.
func generateToken(hmacSecret string) (string, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	nonce64 := base64.RawURLEncoding.EncodeToString(nonce)

	mac := hmac.New(sha256.New, []byte(hmacSecret))
	_, _ = mac.Write([]byte(nonce64))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%s.%s", nonce64, sig), nil
}

// BearerAuthMiddleware validates incoming Bearer tokens (OAuth or headscale API key).
func BearerAuthMiddleware(store *tokenStore, headscaleAPIKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := extractBearerToken(r)
			if tok == "" {
				// Try HTTP Basic Auth with token as username (Tailscale CLI pattern).
				u, _, ok := r.BasicAuth()
				if ok {
					tok = u
				}
			}

			if tok == "" {
				writeError(w, http.StatusUnauthorized, "missing authorization token")
				return
			}

			// Accept headscale API key directly (for Terraform etc).
			if tok == headscaleAPIKey {
				next.ServeHTTP(w, r)
				return
			}

			// Validate OAuth token from store.
			entry, ok := store.lookup(tok)
			if !ok || time.Now().After(entry.expiry) {
				writeError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// TokenStoreKey is a type for context keys used to pass the token store.
type TokenStoreKey struct{}

// MarshalJSON is not used — included to suppress unused import warning.
var _ = json.Marshal
