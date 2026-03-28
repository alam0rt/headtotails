package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSuccess(t *testing.T) {
	t.Setenv("HEADSCALE_ADDR", "127.0.0.1:50443")
	t.Setenv("HEADSCALE_API_KEY", "hskey-api-123")
	t.Setenv("OAUTH_HMAC_SECRET", "hmac-secret")
	t.Setenv("OAUTH_CLIENT_ID", "client-id")
	t.Setenv("OAUTH_CLIENT_SECRET", "client-secret")
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("LOG_ADD_SOURCE", "true")
	t.Setenv("ENVIRONMENT", "test")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, ":8080", cfg.ListenAddr)
	assert.Equal(t, "-", cfg.TailnetName)
	assert.Equal(t, "127.0.0.1:50443", cfg.HeadscaleAddr)
	assert.Equal(t, "warn", cfg.LogLevel)
	assert.True(t, cfg.LogAddSource)
	assert.Equal(t, "test", cfg.Environment)
}

func TestLoadInvalidBool(t *testing.T) {
	t.Setenv("HEADSCALE_ADDR", "127.0.0.1:50443")
	t.Setenv("HEADSCALE_API_KEY", "hskey-api-123")
	t.Setenv("OAUTH_HMAC_SECRET", "hmac-secret")
	t.Setenv("OAUTH_CLIENT_ID", "client-id")
	t.Setenv("OAUTH_CLIENT_SECRET", "client-secret")
	t.Setenv("LOG_ADD_SOURCE", "not-a-bool")

	_, err := Load()
	require.Error(t, err)
}

func TestLoadRejectsEmptyOAuthClientID(t *testing.T) {
	t.Setenv("HEADSCALE_ADDR", "127.0.0.1:50443")
	t.Setenv("HEADSCALE_API_KEY", "hskey-api-123")
	t.Setenv("OAUTH_HMAC_SECRET", "hmac-secret")
	t.Setenv("OAUTH_CLIENT_ID", "")
	t.Setenv("OAUTH_CLIENT_SECRET", "client-secret")

	_, err := Load()
	require.Error(t, err, "empty OAUTH_CLIENT_ID should be rejected")
	assert.Contains(t, err.Error(), "OAUTH_CLIENT_ID")
}

func TestLoadRejectsEmptyOAuthClientSecret(t *testing.T) {
	t.Setenv("HEADSCALE_ADDR", "127.0.0.1:50443")
	t.Setenv("HEADSCALE_API_KEY", "hskey-api-123")
	t.Setenv("OAUTH_HMAC_SECRET", "hmac-secret")
	t.Setenv("OAUTH_CLIENT_ID", "client-id")
	t.Setenv("OAUTH_CLIENT_SECRET", "")

	_, err := Load()
	require.Error(t, err, "empty OAUTH_CLIENT_SECRET should be rejected")
	assert.Contains(t, err.Error(), "OAUTH_CLIENT_SECRET")
}

func TestLoadRejectsEmptyOAuthHMACSecret(t *testing.T) {
	t.Setenv("HEADSCALE_ADDR", "127.0.0.1:50443")
	t.Setenv("HEADSCALE_API_KEY", "hskey-api-123")
	t.Setenv("OAUTH_HMAC_SECRET", "")
	t.Setenv("OAUTH_CLIENT_ID", "client-id")
	t.Setenv("OAUTH_CLIENT_SECRET", "client-secret")

	_, err := Load()
	require.Error(t, err, "empty OAUTH_HMAC_SECRET should be rejected")
	assert.Contains(t, err.Error(), "OAUTH_HMAC_SECRET")
}

func TestLoadRejectsEmptyHeadscaleAPIKey(t *testing.T) {
	t.Setenv("HEADSCALE_ADDR", "127.0.0.1:50443")
	t.Setenv("HEADSCALE_API_KEY", "")
	t.Setenv("OAUTH_HMAC_SECRET", "hmac-secret")
	t.Setenv("OAUTH_CLIENT_ID", "client-id")
	t.Setenv("OAUTH_CLIENT_SECRET", "client-secret")

	_, err := Load()
	require.Error(t, err, "empty HEADSCALE_API_KEY should be rejected")
	assert.Contains(t, err.Error(), "HEADSCALE_API_KEY")
}

// --- WIF config tests ---

// setBaseEnv sets the minimum env for a valid non-WIF config.
func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HEADSCALE_ADDR", "127.0.0.1:50443")
	t.Setenv("HEADSCALE_API_KEY", "hskey-api-123")
	t.Setenv("OAUTH_HMAC_SECRET", "hmac-secret")
	t.Setenv("OAUTH_CLIENT_ID", "client-id")
	t.Setenv("OAUTH_CLIENT_SECRET", "client-secret")
}

func TestLoadWIFDisabledByDefault(t *testing.T) {
	setBaseEnv(t)

	cfg, err := Load()
	require.NoError(t, err)
	assert.False(t, cfg.WIFEnabled)
}

func TestLoadWIFDisabledIgnoresEmptyIssuer(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("WIF_ENABLED", "false")
	t.Setenv("WIF_ISSUER_URL", "")
	t.Setenv("WIF_CLIENT_ID", "")

	cfg, err := Load()
	require.NoError(t, err, "WIF fields should be ignored when WIF_ENABLED=false")
	assert.False(t, cfg.WIFEnabled)
}

func TestLoadWIFEnabledSuccess(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("WIF_ENABLED", "true")
	t.Setenv("WIF_ISSUER_URL", "https://kubernetes.default.svc")
	t.Setenv("WIF_AUDIENCE", "headtotails")
	t.Setenv("WIF_CLIENT_ID", "ts-operator")

	cfg, err := Load()
	require.NoError(t, err)
	assert.True(t, cfg.WIFEnabled)
	assert.Equal(t, "https://kubernetes.default.svc", cfg.WIFIssuerURL)
	assert.Equal(t, "headtotails", cfg.WIFAudience)
	assert.Equal(t, "ts-operator", cfg.WIFClientID)
}

func TestLoadWIFEnabledRejectsEmptyIssuerURL(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("WIF_ENABLED", "true")
	t.Setenv("WIF_ISSUER_URL", "")
	t.Setenv("WIF_CLIENT_ID", "ts-operator")

	_, err := Load()
	require.Error(t, err, "WIF_ENABLED=true with empty WIF_ISSUER_URL should fail")
	assert.Contains(t, err.Error(), "WIF_ISSUER_URL")
}

func TestLoadWIFEnabledRejectsEmptyClientID(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("WIF_ENABLED", "true")
	t.Setenv("WIF_ISSUER_URL", "https://kubernetes.default.svc")
	t.Setenv("WIF_CLIENT_ID", "")

	_, err := Load()
	require.Error(t, err, "WIF_ENABLED=true with empty WIF_CLIENT_ID should fail")
	assert.Contains(t, err.Error(), "WIF_CLIENT_ID")
}

func TestLoadWIFEnabledRejectsBothMissing(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("WIF_ENABLED", "true")
	t.Setenv("WIF_ISSUER_URL", "")
	t.Setenv("WIF_CLIENT_ID", "")

	_, err := Load()
	require.Error(t, err, "WIF_ENABLED=true with both fields empty should fail")
	assert.Contains(t, err.Error(), "WIF_ISSUER_URL")
	assert.Contains(t, err.Error(), "WIF_CLIENT_ID")
}

func TestLoadWIFEnabledAudienceOptional(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("WIF_ENABLED", "true")
	t.Setenv("WIF_ISSUER_URL", "https://kubernetes.default.svc")
	t.Setenv("WIF_CLIENT_ID", "ts-operator")
	// WIF_AUDIENCE intentionally not set

	cfg, err := Load()
	require.NoError(t, err, "WIF_AUDIENCE should be optional")
	assert.True(t, cfg.WIFEnabled)
	assert.Empty(t, cfg.WIFAudience)
}
