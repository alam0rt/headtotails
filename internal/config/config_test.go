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
