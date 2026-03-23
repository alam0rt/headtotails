package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alam0rt/headtotails/internal/headscale"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlogMiddlewareLogsStructuredRequestFields(t *testing.T) {
	old := slog.Default()
	defer slog.SetDefault(old)

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	router, _ := setupTestRouter(&headscale.MockHeadscaleClient{})
	r := httptest.NewRequest(http.MethodGet, "/healthz?token=secret", nil)
	r.RemoteAddr = "203.0.113.1:40000"
	r.Header.Set("User-Agent", "unit-test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	logLine := firstLogLine(t, buf.Bytes())
	assert.Equal(t, "request", logLine["msg"])
	assert.Equal(t, "GET", logLine["method"])
	assert.Equal(t, "/healthz", logLine["route"])
	assert.Equal(t, "203.0.113.1", logLine["remote_ip"])
	assert.Equal(t, "unit-test-agent", logLine["user_agent"])
	assert.EqualValues(t, float64(http.StatusOK), logLine["http_status"])
	assert.NotEmpty(t, logLine["request_id"])
	assert.Contains(t, logLine, "duration_ms")
	assert.Contains(t, logLine, "bytes_written")
	assert.NotContains(t, logLine, "path")
	assert.NotContains(t, logLine, "token")
}

func firstLogLine(t *testing.T, data []byte) map[string]any {
	t.Helper()

	line, err := io.ReadAll(bytes.NewReader(bytes.TrimSpace(data)))
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(line, &got))
	return got
}
