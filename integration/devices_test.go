package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeviceListAndDelete tests the device list and delete flow.
func TestDeviceListAndDelete(t *testing.T) {
	IntegrationSkip(t)

	base := sharedStack.endpoint + "/api/v2"
	client := &http.Client{Timeout: 10 * time.Second}

	token := mustGetToken(t)
	authHeader := "Bearer " + token

	// List devices.
	req := mustNewRequest(t, http.MethodGet, base+"/tailnet/-/devices", nil, authHeader)
	resp := mustDo(t, client, req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var list struct {
		Devices []struct {
			ID       string `json:"id"`
			Hostname string `json:"hostname"`
		} `json:"devices"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
	resp.Body.Close()

	// If any device exists, verify we can get and delete it.
	if len(list.Devices) > 0 {
		deviceID := list.Devices[0].ID

		// Get device.
		req = mustNewRequest(t, http.MethodGet,
			fmt.Sprintf("%s/device/%s", base, deviceID), nil, authHeader)
		resp = mustDo(t, client, req)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var device struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&device))
		resp.Body.Close()
		assert.Equal(t, deviceID, device.ID)

		// Delete device.
		req = mustNewRequest(t, http.MethodDelete,
			fmt.Sprintf("%s/device/%s", base, deviceID), nil, authHeader)
		resp = mustDo(t, client, req)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}
}
