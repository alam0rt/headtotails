package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWriteGRPCErrorLogsWarnFor4xx(t *testing.T) {
	old := slog.Default()
	defer slog.SetDefault(old)

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	req := httptest.NewRequest(http.MethodGet, "/api/v2/device/123", nil)
	rec := httptest.NewRecorder()

	writeGRPCError(rec, req, status.Error(codes.NotFound, "missing node"))

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var line map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line))
	assert.Equal(t, "WARN", line["level"])
	assert.Equal(t, "headscale", line["upstream"])
	assert.Equal(t, float64(http.StatusNotFound), line["http_status"])
	assert.Equal(t, "NotFound", line["grpc_code"])
}

func TestWriteGRPCErrorLogsErrorFor5xx(t *testing.T) {
	old := slog.Default()
	defer slog.SetDefault(old)

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	req := httptest.NewRequest(http.MethodGet, "/api/v2/device/123", nil)
	rec := httptest.NewRecorder()

	writeGRPCError(rec, req, status.Error(codes.Internal, "backend exploded"))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var line map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line))
	assert.Equal(t, "ERROR", line["level"])
	assert.Equal(t, "headscale", line["upstream"])
	assert.Equal(t, float64(http.StatusInternalServerError), line["http_status"])
	assert.Equal(t, "Internal", line["grpc_code"])
}
