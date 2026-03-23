package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/juanfont/headscale/gen/go/headscale/v1"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/alam0rt/headtotails/internal/headscale"
	"github.com/alam0rt/headtotails/internal/model"
)

func TestListKeys(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("ListUsers", mock.Anything).Return([]*v1.User{
		{Id: 1, Name: "default"},
	}, nil)
	m.On("ListPreAuthKeys", mock.Anything, "default").Return([]*v1.PreAuthKey{
		{Id: 1, Key: "abc123", Reusable: false, Ephemeral: true},
		{Id: 2, Key: "def456", Reusable: true, Ephemeral: false},
	}, nil)

	router, tok := setupTestRouter(m)
	w := doRequest(router, http.MethodGet, "/api/v2/tailnet/-/keys", tok)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp model.KeyList
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Len(t, resp.Keys, 2)
	assert.Equal(t, "1", resp.Keys[0].ID)
	assert.Equal(t, "2", resp.Keys[1].ID)
	m.AssertExpectations(t)
}

func TestCreateKey(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("ListUsers", mock.Anything).Return([]*v1.User{
		{Id: 1, Name: "default"},
	}, nil)
	m.On("CreatePreAuthKey", mock.Anything, mock.MatchedBy(func(req *v1.CreatePreAuthKeyRequest) bool {
		return req.User == 1 && req.Ephemeral
	})).Return(&v1.PreAuthKey{
		Id:        42,
		Key:       "hskey-auth-newkey123",
		Ephemeral: true,
	}, nil)

	router, tok := setupTestRouter(m)

	body, _ := json.Marshal(model.CreateKeyRequest{
		Capabilities: model.KeyCapability{
			Devices: model.KeyCapabilityDevices{
				Create: model.KeyCapabilityDevicesCreate{
					Ephemeral:     true,
					Preauthorized: true,
				},
			},
		},
		ExpirySeconds: 3600,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v2/tailnet/-/keys", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp model.Key
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "42", resp.ID)
	assert.True(t, strings.HasPrefix(resp.Key, "hskey-auth-") || strings.HasPrefix(resp.Key, "tskey-auth-"),
		"expected key prefix, got %q", resp.Key)
	m.AssertExpectations(t)
}

func TestCreateKeyInvalidBody(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("ListUsers", mock.Anything).Return([]*v1.User{
		{Id: 1, Name: "default"},
	}, nil)

	router, tok := setupTestRouter(m)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/tailnet/-/keys", strings.NewReader(`{"capabilities":`))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	m.AssertExpectations(t)
}

func TestCreateKeyGRPCError(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("ListUsers", mock.Anything).Return([]*v1.User{
		{Id: 1, Name: "default"},
	}, nil)
	m.On("CreatePreAuthKey", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.Internal, "create failed"))

	router, tok := setupTestRouter(m)
	body := `{"capabilities":{"devices":{"create":{"ephemeral":true}}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v2/tailnet/-/keys", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	m.AssertExpectations(t)
}

func TestGetKey(t *testing.T) {
	tests := []struct {
		name       string
		keyID      string
		mockSetup  func(*headscale.MockHeadscaleClient)
		wantStatus int
	}{
		{
			name:  "found",
			keyID: "1",
			mockSetup: func(m *headscale.MockHeadscaleClient) {
				m.On("ListUsers", mock.Anything).Return([]*v1.User{
					{Id: 1, Name: "default"},
				}, nil)
				m.On("ListPreAuthKeys", mock.Anything, "default").Return([]*v1.PreAuthKey{
					{Id: 1, Key: "k1"},
				}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:  "not found",
			keyID: "999",
			mockSetup: func(m *headscale.MockHeadscaleClient) {
				m.On("ListUsers", mock.Anything).Return([]*v1.User{
					{Id: 1, Name: "default"},
				}, nil)
				m.On("ListPreAuthKeys", mock.Anything, "default").Return([]*v1.PreAuthKey{
					{Id: 1, Key: "k1"},
				}, nil)
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &headscale.MockHeadscaleClient{}
			tt.mockSetup(m)
			router, tok := setupTestRouter(m)

			w := doRequest(router, http.MethodGet,
				fmt.Sprintf("/api/v2/tailnet/-/keys/%s", tt.keyID), tok)
			assert.Equal(t, tt.wantStatus, w.Code)
			m.AssertExpectations(t)
		})
	}
}

func TestDeleteKey(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("ListUsers", mock.Anything).Return([]*v1.User{
		{Id: 1, Name: "default"},
	}, nil)
	m.On("ListPreAuthKeys", mock.Anything, "default").Return([]*v1.PreAuthKey{
		{Id: 5, Key: "k5"},
	}, nil)
	m.On("ExpirePreAuthKey", mock.Anything, uint64(5)).Return(nil)
	m.On("DeletePreAuthKey", mock.Anything, uint64(5)).Return(nil)

	router, tok := setupTestRouter(m)
	w := doRequest(router, http.MethodDelete, "/api/v2/tailnet/-/keys/5", tok)

	assert.Equal(t, http.StatusOK, w.Code)
	m.AssertExpectations(t)
}

func TestDeleteKeyNotFound(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("ListUsers", mock.Anything).Return([]*v1.User{
		{Id: 1, Name: "default"},
	}, nil)
	m.On("ListPreAuthKeys", mock.Anything, "default").Return([]*v1.PreAuthKey{}, nil)

	router, tok := setupTestRouter(m)
	w := doRequest(router, http.MethodDelete, "/api/v2/tailnet/-/keys/999", tok)

	assert.Equal(t, http.StatusNotFound, w.Code)
	m.AssertExpectations(t)
}

func TestDeleteKeyExpireError(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("ListUsers", mock.Anything).Return([]*v1.User{
		{Id: 1, Name: "default"},
	}, nil)
	m.On("ListPreAuthKeys", mock.Anything, "default").Return([]*v1.PreAuthKey{
		{Id: 5, Key: "k5"},
	}, nil)
	m.On("ExpirePreAuthKey", mock.Anything, uint64(5)).
		Return(status.Error(codes.Internal, "expire failed"))

	router, tok := setupTestRouter(m)
	w := doRequest(router, http.MethodDelete, "/api/v2/tailnet/-/keys/5", tok)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	m.AssertNotCalled(t, "DeletePreAuthKey", mock.Anything, mock.Anything)
	m.AssertExpectations(t)
}

func TestDeleteKeyDeleteError(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("ListUsers", mock.Anything).Return([]*v1.User{
		{Id: 1, Name: "default"},
	}, nil)
	m.On("ListPreAuthKeys", mock.Anything, "default").Return([]*v1.PreAuthKey{
		{Id: 5, Key: "k5"},
	}, nil)
	m.On("ExpirePreAuthKey", mock.Anything, uint64(5)).Return(nil)
	m.On("DeletePreAuthKey", mock.Anything, uint64(5)).
		Return(status.Error(codes.Internal, "delete failed"))

	router, tok := setupTestRouter(m)
	w := doRequest(router, http.MethodDelete, "/api/v2/tailnet/-/keys/5", tok)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	m.AssertExpectations(t)
}

func TestPutKeyNotImplemented(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	router, tok := setupTestRouter(m)

	req := httptest.NewRequest(http.MethodPut, "/api/v2/tailnet/-/keys/1", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
	var errResp model.Error
	require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
	assert.Equal(t, "OAuth client and federated identity keys are Tailscale SaaS features", errResp.Message)
}

func TestListKeysGRPCError(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("ListUsers", mock.Anything).Return([]*v1.User{
		{Id: 1, Name: "default"},
	}, nil)
	m.On("ListPreAuthKeys", mock.Anything, "default").
		Return(nil, status.Error(codes.Internal, "grpc down"))

	router, tok := setupTestRouter(m)
	before := testutil.ToFloat64(grpcErrorsTotal.WithLabelValues(codes.Internal.String()))
	w := doRequest(router, http.MethodGet, "/api/v2/tailnet/-/keys", tok)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	after := testutil.ToFloat64(grpcErrorsTotal.WithLabelValues(codes.Internal.String()))
	assert.Equal(t, before+1, after)
	m.AssertExpectations(t)
}

func TestKeysTailnetIsolation(t *testing.T) {
	t.Run("rejects mismatched explicit tailnet", func(t *testing.T) {
		m := &headscale.MockHeadscaleClient{}
		m.On("ListUsers", mock.Anything).Return([]*v1.User{
			{Id: 1, Name: "default"},
		}, nil)

		router, tok := setupTestRouter(m)
		w := doRequest(router, http.MethodGet, "/api/v2/tailnet/other/keys", tok)
		assert.Equal(t, http.StatusNotFound, w.Code)
		m.AssertExpectations(t)
	})

	t.Run("fails when default tailnet is ambiguous", func(t *testing.T) {
		m := &headscale.MockHeadscaleClient{}
		m.On("ListUsers", mock.Anything).Return([]*v1.User{
			{Id: 1, Name: "alice"},
			{Id: 2, Name: "bob"},
		}, nil)

		router, tok := setupTestRouter(m)
		w := doRequest(router, http.MethodGet, "/api/v2/tailnet/-/keys", tok)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		m.AssertExpectations(t)
	})

	t.Run("create uses requested tailnet user id", func(t *testing.T) {
		m := &headscale.MockHeadscaleClient{}
		m.On("ListUsers", mock.Anything).Return([]*v1.User{
			{Id: 10, Name: "alice"},
			{Id: 20, Name: "bob"},
		}, nil)
		m.On("CreatePreAuthKey", mock.Anything, mock.MatchedBy(func(req *v1.CreatePreAuthKeyRequest) bool {
			return req.GetUser() == 20
		})).Return(&v1.PreAuthKey{
			Id: 100,
		}, nil)

		router, tok := setupTestRouter(m)
		body := `{"capabilities":{"devices":{"create":{"reusable":false,"ephemeral":true,"preauthorized":true}}},"expirySeconds":3600}`
		req := httptest.NewRequest(http.MethodPost, "/api/v2/tailnet/bob/keys", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		m.AssertExpectations(t)
	})
}

func TestKeyResolveErrorErrorMethod(t *testing.T) {
	err := &keyResolveError{status: http.StatusBadRequest, msg: "boom"}
	assert.Equal(t, "boom", err.Error())
}

func TestWriteResolveErrorFallsBackToGRPCError(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/tailnet/-/keys", nil)
	writeResolveError(w, req, status.Error(codes.Internal, "upstream"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestResolveTailnetUserNoUsers(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("ListUsers", mock.Anything).Return([]*v1.User{}, nil)
	h := &keysHandler{hs: m, tailnetName: "-"}

	user, err := h.resolveTailnetUser(context.Background(), "-")
	assert.Nil(t, user)
	require.Error(t, err)
	var resolveErr *keyResolveError
	require.True(t, errors.As(err, &resolveErr))
	assert.Equal(t, http.StatusNotFound, resolveErr.status)
	m.AssertExpectations(t)
}
