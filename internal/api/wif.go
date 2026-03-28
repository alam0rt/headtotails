package api

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// JWTValidator validates JWTs from a trusted OIDC issuer.
type JWTValidator interface {
	// Validate parses and validates the JWT, returning the claims if valid.
	Validate(ctx context.Context, rawJWT string) (*JWTClaims, error)
}

// JWTClaims holds standard JWT claims we care about for WIF.
type JWTClaims struct {
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"`
	Audience  any    `json:"aud"` // string or []string
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
}

// AudienceContains checks if the claims contain the given audience.
func (c *JWTClaims) AudienceContains(aud string) bool {
	switch v := c.Audience.(type) {
	case string:
		return v == aud
	case []any:
		for _, a := range v {
			if s, ok := a.(string); ok && s == aud {
				return true
			}
		}
	}
	return false
}

// OIDCValidator validates JWTs by fetching JWKS from the issuer's
// OIDC discovery endpoint.
type OIDCValidator struct {
	issuerURL  string
	audience   string // expected audience (optional)
	clientID   string // expected client_id
	httpClient *http.Client

	mu   sync.RWMutex
	jwks *jwksCache
}

type jwksCache struct {
	keys      map[string]crypto.PublicKey // kid → public key
	fetchedAt time.Time
}

const jwksCacheTTL = 1 * time.Hour

// NewOIDCValidator creates a validator for the given issuer.
func NewOIDCValidator(issuerURL, audience, clientID string, httpClient *http.Client) *OIDCValidator {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &OIDCValidator{
		issuerURL:  strings.TrimRight(issuerURL, "/"),
		audience:   audience,
		clientID:   clientID,
		httpClient: httpClient,
	}
}

// Validate verifies the JWT signature, expiry, issuer, and optionally audience.
func (v *OIDCValidator) Validate(ctx context.Context, rawJWT string) (*JWTClaims, error) {
	// 1. Decode header to get kid + alg.
	parts := strings.SplitN(rawJWT, ".", 3)
	if len(parts) != 3 {
		return nil, errors.New("wif: malformed JWT: expected 3 parts")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("wif: invalid JWT header encoding: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("wif: invalid JWT header: %w", err)
	}

	// 2. Get the signing key.
	key, err := v.getKey(ctx, header.Kid)
	if err != nil {
		return nil, fmt.Errorf("wif: failed to get signing key: %w", err)
	}

	// 3. Verify signature.
	signed := parts[0] + "." + parts[1]
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("wif: invalid JWT signature encoding: %w", err)
	}

	if err := verifySignature(header.Alg, key, []byte(signed), sigBytes); err != nil {
		return nil, fmt.Errorf("wif: signature verification failed: %w", err)
	}

	// 4. Decode claims.
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("wif: invalid JWT claims encoding: %w", err)
	}
	var claims JWTClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("wif: invalid JWT claims: %w", err)
	}

	// 5. Validate claims.
	now := time.Now().Unix()
	if claims.ExpiresAt != 0 && now > claims.ExpiresAt {
		return nil, errors.New("wif: token expired")
	}
	if claims.Issuer != v.issuerURL {
		return nil, fmt.Errorf("wif: issuer mismatch: got %q, want %q", claims.Issuer, v.issuerURL)
	}
	if v.audience != "" && !claims.AudienceContains(v.audience) {
		return nil, fmt.Errorf("wif: audience %q not found in token", v.audience)
	}

	return &claims, nil
}

func (v *OIDCValidator) getKey(ctx context.Context, kid string) (crypto.PublicKey, error) {
	// Try cache first.
	v.mu.RLock()
	if v.jwks != nil && time.Since(v.jwks.fetchedAt) < jwksCacheTTL {
		if key, ok := v.jwks.keys[kid]; ok {
			v.mu.RUnlock()
			return key, nil
		}
	}
	v.mu.RUnlock()

	// Refresh JWKS.
	if err := v.refreshJWKS(ctx); err != nil {
		return nil, err
	}

	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok := v.jwks.keys[kid]
	if !ok {
		return nil, fmt.Errorf("kid %q not found in JWKS", kid)
	}
	return key, nil
}

func (v *OIDCValidator) refreshJWKS(ctx context.Context) error {
	// Discover JWKS URI.
	jwksURI, err := v.discoverJWKSURI(ctx)
	if err != nil {
		return err
	}

	// Fetch JWKS.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return fmt.Errorf("wif: build JWKS request: %w", err)
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("wif: fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wif: JWKS endpoint returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("wif: read JWKS body: %w", err)
	}

	var jwks struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		return fmt.Errorf("wif: parse JWKS: %w", err)
	}

	keys := make(map[string]crypto.PublicKey, len(jwks.Keys))
	for _, raw := range jwks.Keys {
		var jwk struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			// RSA
			N string `json:"n"`
			E string `json:"e"`
			// EC
			Crv string `json:"crv"`
			X   string `json:"x"`
			Y   string `json:"y"`
		}
		if err := json.Unmarshal(raw, &jwk); err != nil {
			continue
		}
		switch jwk.Kty {
		case "RSA":
			key, err := parseRSAPublicKey(jwk.N, jwk.E)
			if err != nil {
				continue
			}
			keys[jwk.Kid] = key
		case "EC":
			key, err := parseECPublicKey(jwk.Crv, jwk.X, jwk.Y)
			if err != nil {
				continue
			}
			keys[jwk.Kid] = key
		}
	}

	v.mu.Lock()
	v.jwks = &jwksCache{keys: keys, fetchedAt: time.Now()}
	v.mu.Unlock()
	return nil
}

func (v *OIDCValidator) discoverJWKSURI(ctx context.Context) (string, error) {
	url := v.issuerURL + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("wif: build discovery request: %w", err)
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("wif: OIDC discovery fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wif: OIDC discovery returned %d", resp.StatusCode)
	}

	var discovery struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return "", fmt.Errorf("wif: parse OIDC discovery: %w", err)
	}
	if discovery.JWKSURI == "" {
		return "", errors.New("wif: OIDC discovery missing jwks_uri")
	}
	return discovery.JWKSURI, nil
}

// verifySignature verifies a JWT signature for supported algorithms.
func verifySignature(alg string, key crypto.PublicKey, signed, sig []byte) error {
	switch alg {
	case "RS256":
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return errors.New("key is not RSA")
		}
		h := sha256.Sum256(signed)
		return rsa.VerifyPKCS1v15(rsaKey, crypto.SHA256, h[:], sig)

	case "ES256":
		ecKey, ok := key.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("key is not ECDSA")
		}
		h := sha256.Sum256(signed)
		if !ecdsa.VerifyASN1(ecKey, h[:], sig) {
			return errors.New("ECDSA signature verification failed")
		}
		return nil

	default:
		return fmt.Errorf("unsupported algorithm: %s", alg)
	}
}

// parseRSAPublicKey builds an RSA public key from JWK n and e values.
func parseRSAPublicKey(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, err
	}
	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}

// parseECPublicKey builds an ECDSA public key from JWK curve, x, y values.
func parseECPublicKey(crv, xB64, yB64 string) (*ecdsa.PublicKey, error) {
	var curve elliptic.Curve
	switch crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	default:
		return nil, fmt.Errorf("unsupported curve: %s", crv)
	}
	xBytes, err := base64.RawURLEncoding.DecodeString(xB64)
	if err != nil {
		return nil, err
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(yB64)
	if err != nil {
		return nil, err
	}
	return &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}, nil
}

// TokenExchangeHandler returns an http.HandlerFunc for POST /oauth/token-exchange.
// It validates an incoming JWT against the configured OIDC issuer and, if valid,
// issues a bearer token via the existing tokenStore.
func TokenExchangeHandler(validator JWTValidator, hmacSecret, expectedClientID string, store *tokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if validator == nil {
			writeError(w, http.StatusNotImplemented, "workload identity federation is not configured")
			return
		}

		grantType := r.FormValue("grant_type")
		if grantType != "client_credentials" {
			writeError(w, http.StatusBadRequest, "unsupported grant_type; only client_credentials is supported")
			return
		}

		// The operator sends client_id via Basic Auth (with empty password) or form param.
		clientID, _, _ := r.BasicAuth()
		if clientID == "" {
			clientID = r.FormValue("client_id")
		}
		if clientID == "" {
			writeError(w, http.StatusBadRequest, "missing client_id")
			return
		}
		if clientID != expectedClientID {
			writeError(w, http.StatusUnauthorized, "invalid client_id")
			return
		}

		jwt := r.FormValue("jwt")
		if jwt == "" {
			writeError(w, http.StatusBadRequest, "missing jwt parameter")
			return
		}

		claims, err := validator.Validate(r.Context(), jwt)
		if err != nil {
			writeError(w, http.StatusUnauthorized, fmt.Sprintf("invalid JWT: %v", err))
			return
		}
		_ = claims // claims validated; could log subject etc.

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
