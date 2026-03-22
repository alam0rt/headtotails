package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "github.com/juanfont/headscale/gen/go/headscale/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/alam0rt/headtotails/internal/headscale"
	"github.com/alam0rt/headtotails/internal/model"
)

func TestListUsers(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("ListUsers", mock.Anything).Return([]*v1.User{
		{Id: 1, Name: "alice"},
		{Id: 2, Name: "bob"},
	}, nil)

	router, tok := setupTestRouter(m)
	w := doRequest(router, http.MethodGet, "/api/v2/tailnet/-/users", tok)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp model.UserList
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Len(t, resp.Users, 2)
	assert.Equal(t, "1", resp.Users[0].ID)
	m.AssertExpectations(t)
}

func TestGetUser(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		mockSetup  func(*headscale.MockHeadscaleClient)
		wantStatus int
	}{
		{
			name:   "found",
			userID: "1",
			mockSetup: func(m *headscale.MockHeadscaleClient) {
				m.On("ListUsers", mock.Anything).Return([]*v1.User{
					{Id: 1, Name: "alice"},
				}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "not found",
			userID: "999",
			mockSetup: func(m *headscale.MockHeadscaleClient) {
				m.On("ListUsers", mock.Anything).Return([]*v1.User{
					{Id: 1, Name: "alice"},
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

			w := doRequest(router, http.MethodGet, "/api/v2/users/"+tt.userID, tok)
			assert.Equal(t, tt.wantStatus, w.Code)
			m.AssertExpectations(t)
		})
	}
}

func TestDeleteUser(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("DeleteUser", mock.Anything, uint64(3)).Return(nil)

	router, tok := setupTestRouter(m)
	w := doRequest(router, http.MethodPost, "/api/v2/users/3/delete", tok)

	assert.Equal(t, http.StatusOK, w.Code)
	m.AssertExpectations(t)
}

func TestDeleteUserNotFound(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("DeleteUser", mock.Anything, uint64(99)).
		Return(status.Error(codes.NotFound, "user not found"))

	router, tok := setupTestRouter(m)
	w := doRequest(router, http.MethodPost, "/api/v2/users/99/delete", tok)

	assert.Equal(t, http.StatusNotFound, w.Code)
	m.AssertExpectations(t)
}

func TestUserStubEndpoints(t *testing.T) {
	stubs := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v2/users/1/approve"},
		{http.MethodPost, "/api/v2/users/1/suspend"},
		{http.MethodPost, "/api/v2/users/1/restore"},
		{http.MethodPost, "/api/v2/users/1/role"},
	}

	m := &headscale.MockHeadscaleClient{}
	router, tok := setupTestRouter(m)

	for _, s := range stubs {
		t.Run(s.method+" "+s.path, func(t *testing.T) {
			req, _ := http.NewRequest(s.method, s.path, nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusNotImplemented, w.Code)
		})
	}
}
