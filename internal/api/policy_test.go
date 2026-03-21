package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/alam0rt/headtotail/internal/headscale"
)

func TestGetPolicy(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("GetPolicy", mock.Anything).Return(`{"acls":[{"action":"accept","src":["*"],"dst":["*:*"]}]}`, nil)

	router, tok := setupTestRouter(m)
	w := doRequest(router, http.MethodGet, "/api/v2/tailnet/-/acl", tok)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotEmpty(t, resp["policy"])
	m.AssertExpectations(t)
}

func TestGetPolicyHuJSON(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("GetPolicy", mock.Anything).Return(`{"acls":[]}`, nil)

	router, tok := setupTestRouter(m)
	req := httptest.NewRequest(http.MethodGet, "/api/v2/tailnet/-/acl", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/hujson")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/hujson", w.Header().Get("Content-Type"))
	m.AssertExpectations(t)
}

func TestSetPolicy(t *testing.T) {
	policy := `{"acls":[{"action":"accept","src":["*"],"dst":["*:*"]}]}`
	m := &headscale.MockHeadscaleClient{}
	m.On("SetPolicy", mock.Anything, policy).Return(nil)

	router, tok := setupTestRouter(m)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/tailnet/-/acl",
		bytes.NewBufferString(policy))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	m.AssertExpectations(t)
}

func TestGetPolicyGRPCError(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("GetPolicy", mock.Anything).
		Return("", status.Error(codes.Internal, "grpc error"))

	router, tok := setupTestRouter(m)
	w := doRequest(router, http.MethodGet, "/api/v2/tailnet/-/acl", tok)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	m.AssertExpectations(t)
}

func TestPolicyStubEndpoints(t *testing.T) {
	stubs := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v2/tailnet/-/acl/preview"},
		{http.MethodPost, "/api/v2/tailnet/-/acl/validate"},
	}

	m := &headscale.MockHeadscaleClient{}
	router, tok := setupTestRouter(m)

	for _, s := range stubs {
		t.Run(s.method+" "+s.path, func(t *testing.T) {
			req := httptest.NewRequest(s.method, s.path, nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusNotImplemented, w.Code)
		})
	}
}
