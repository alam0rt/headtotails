package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthKeyRoundTrip tests creating, listing, and deleting an auth key.
func TestAuthKeyRoundTrip(t *testing.T) {
	IntegrationSkip(t)
	t.Skip("TestAuthKeyRoundTrip: full Docker stack not wired")

	endpoint := "http://localhost:8080"
	base := endpoint + "/api/v2"
	client := &http.Client{Timeout: 10 * time.Second}

	token := mustGetOAuthToken(t, endpoint, "test-client", "test-secret")
	authHeader := "Bearer " + token

	// Create key.
	keyBody := map[string]interface{}{
		"capabilities": map[string]interface{}{
			"devices": map[string]interface{}{
				"create": map[string]interface{}{
					"reusable":      false,
					"ephemeral":     true,
					"preauthorized": true,
				},
			},
		},
		"expirySeconds": 3600,
	}

	req := mustNewRequest(t, http.MethodPost, base+"/tailnet/-/keys", keyBody, authHeader)
	resp := mustDo(t, client, req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var created struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	resp.Body.Close()
	assert.NotEmpty(t, created.ID)

	// List keys — created key should appear.
	req = mustNewRequest(t, http.MethodGet, base+"/tailnet/-/keys", nil, authHeader)
	resp = mustDo(t, client, req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var list struct {
		Keys []struct {
			ID string `json:"id"`
		} `json:"keys"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
	resp.Body.Close()

	found := false
	for _, k := range list.Keys {
		if k.ID == created.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "created key should appear in list")

	// Get specific key.
	req = mustNewRequest(t, http.MethodGet, base+"/tailnet/-/keys/"+created.ID, nil, authHeader)
	resp = mustDo(t, client, req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Delete key.
	req = mustNewRequest(t, http.MethodDelete, base+"/tailnet/-/keys/"+created.ID, nil, authHeader)
	resp = mustDo(t, client, req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify key is gone.
	req = mustNewRequest(t, http.MethodGet, base+"/tailnet/-/keys/"+created.ID, nil, authHeader)
	resp = mustDo(t, client, req)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}
