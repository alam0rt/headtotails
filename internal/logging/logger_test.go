package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactAttr(t *testing.T) {
	t.Parallel()

	cases := []struct {
		key      string
		value    string
		redacted bool
	}{
		{key: "authorization", value: "Bearer secret", redacted: true},
		{key: "client_secret", value: "super-secret", redacted: true},
		{key: "x-api-key", value: "abc123", redacted: true},
		{key: "oauthToken", value: "xyz", redacted: true},
		{key: "request_id", value: "rid-1", redacted: false},
		{key: "route", value: "/api/v2/device/{deviceId}", redacted: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()
			attr := slog.String(tc.key, tc.value)
			got := RedactAttr(nil, attr)
			if tc.redacted {
				assert.Equal(t, redactedValue, got.Value.String())
				return
			}
			assert.Equal(t, tc.value, got.Value.String())
		})
	}
}

func TestParseLevelAliases(t *testing.T) {
	t.Parallel()

	assert.Equal(t, slog.LevelError, parseLevel("err"))
	assert.Equal(t, slog.LevelError, parseLevel("fatal"))
}

func TestNewLoggerProducesJSONAndServiceAttrs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger, err := New(Options{
		Level:   "debug",
		Service: "headtotails",
		Version: "test",
		Env:     "test",
		Output:  &buf,
	})
	require.NoError(t, err)

	logger.Info("hello", "authorization", "Bearer secret", "request_id", "rid-123")

	var line map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line))
	assert.Equal(t, "hello", line["msg"])
	assert.Equal(t, "headtotails", line["service"])
	assert.Equal(t, "test", line["version"])
	assert.Equal(t, "test", line["env"])
	assert.Equal(t, "rid-123", line["request_id"])
	assert.Equal(t, redactedValue, line["authorization"])
}

func TestSetupInstallsDefaultLogger(t *testing.T) {
	old := slog.Default()
	defer slog.SetDefault(old)

	var buf bytes.Buffer
	err := Setup(Options{
		Level:   "info",
		Service: "headtotails",
		Output:  &buf,
	})
	require.NoError(t, err)

	slog.Info("setup-check", "token", "secret")
	var line map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line))
	assert.Equal(t, "setup-check", line["msg"])
	assert.Equal(t, redactedValue, line["token"])
}
