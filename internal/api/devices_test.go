package api

import (
	"encoding/json"
	"net/http"
	"testing"

	v1 "github.com/juanfont/headscale/gen/go/headscale/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/alam0rt/headtotail/internal/headscale"
	"github.com/alam0rt/headtotail/internal/model"
)

func TestListDevices(t *testing.T) {
	tests := []struct {
		name       string
		mockSetup  func(*headscale.MockHeadscaleClient)
		wantStatus int
		wantLen    int
	}{
		{
			name: "empty list",
			mockSetup: func(m *headscale.MockHeadscaleClient) {
				m.On("ListNodes", mock.Anything, "").Return([]*v1.Node{}, nil)
			},
			wantStatus: http.StatusOK,
			wantLen:    0,
		},
		{
			name: "two devices",
			mockSetup: func(m *headscale.MockHeadscaleClient) {
				m.On("ListNodes", mock.Anything, "").Return([]*v1.Node{
					{Id: 1, Name: "node1", IpAddresses: []string{"100.64.0.1"}},
					{Id: 2, Name: "node2", IpAddresses: []string{"100.64.0.2"}},
				}, nil)
			},
			wantStatus: http.StatusOK,
			wantLen:    2,
		},
		{
			name: "gRPC error",
			mockSetup: func(m *headscale.MockHeadscaleClient) {
				m.On("ListNodes", mock.Anything, "").
					Return(nil, status.Error(codes.Internal, "grpc error"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &headscale.MockHeadscaleClient{}
			tt.mockSetup(m)
			router, tok := setupTestRouter(m)

			w := doRequest(router, http.MethodGet, "/api/v2/tailnet/-/devices", tok)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				var resp model.DeviceList
				require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
				assert.Len(t, resp.Devices, tt.wantLen)
			}
			m.AssertExpectations(t)
		})
	}
}

func TestGetDevice(t *testing.T) {
	tests := []struct {
		name       string
		deviceID   string
		mockSetup  func(*headscale.MockHeadscaleClient)
		wantStatus int
		wantID     string
	}{
		{
			name:     "existing device",
			deviceID: "1",
			mockSetup: func(m *headscale.MockHeadscaleClient) {
				m.On("GetNode", mock.Anything, uint64(1)).
					Return(&v1.Node{Id: 1, Name: "mydevice"}, nil)
			},
			wantStatus: http.StatusOK,
			wantID:     "1",
		},
		{
			name:     "not found",
			deviceID: "999",
			mockSetup: func(m *headscale.MockHeadscaleClient) {
				m.On("GetNode", mock.Anything, uint64(999)).
					Return(nil, status.Error(codes.NotFound, "not found"))
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid id",
			deviceID:   "abc",
			mockSetup:  func(m *headscale.MockHeadscaleClient) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:     "permission denied",
			deviceID: "5",
			mockSetup: func(m *headscale.MockHeadscaleClient) {
				m.On("GetNode", mock.Anything, uint64(5)).
					Return(nil, status.Error(codes.PermissionDenied, "denied"))
			},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &headscale.MockHeadscaleClient{}
			tt.mockSetup(m)
			router, tok := setupTestRouter(m)

			w := doRequest(router, http.MethodGet, "/api/v2/device/"+tt.deviceID, tok)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantID != "" {
				var d model.Device
				require.NoError(t, json.NewDecoder(w.Body).Decode(&d))
				assert.Equal(t, tt.wantID, d.ID)
			}
			m.AssertExpectations(t)
		})
	}
}

func TestDeleteDevice(t *testing.T) {
	tests := []struct {
		name       string
		deviceID   string
		mockSetup  func(*headscale.MockHeadscaleClient)
		wantStatus int
	}{
		{
			name:     "success",
			deviceID: "1",
			mockSetup: func(m *headscale.MockHeadscaleClient) {
				m.On("DeleteNode", mock.Anything, uint64(1)).Return(nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "not found",
			deviceID: "99",
			mockSetup: func(m *headscale.MockHeadscaleClient) {
				m.On("DeleteNode", mock.Anything, uint64(99)).
					Return(status.Error(codes.NotFound, "not found"))
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &headscale.MockHeadscaleClient{}
			tt.mockSetup(m)
			router, tok := setupTestRouter(m)

			w := doRequest(router, http.MethodDelete, "/api/v2/device/"+tt.deviceID, tok)
			assert.Equal(t, tt.wantStatus, w.Code)
			m.AssertExpectations(t)
		})
	}
}

func TestUnauthorizedRequest(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	router, _ := setupTestRouter(m)

	w := doRequest(router, http.MethodGet, "/api/v2/tailnet/-/devices", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetDeviceRoutes(t *testing.T) {
	m := &headscale.MockHeadscaleClient{}
	m.On("GetNode", mock.Anything, uint64(1)).Return(&v1.Node{
		Id:             1,
		ApprovedRoutes: []string{"10.0.0.0/8"},
		AvailableRoutes: []string{"10.0.0.0/8", "192.168.1.0/24"},
	}, nil)

	router, tok := setupTestRouter(m)
	w := doRequest(router, http.MethodGet, "/api/v2/device/1/routes", tok)

	assert.Equal(t, http.StatusOK, w.Code)
	var routes model.DeviceRoutes
	require.NoError(t, json.NewDecoder(w.Body).Decode(&routes))
	assert.Equal(t, []string{"10.0.0.0/8", "192.168.1.0/24"}, routes.AdvertisedRoutes)
	assert.Equal(t, []string{"10.0.0.0/8"}, routes.EnabledRoutes)
	m.AssertExpectations(t)
}
