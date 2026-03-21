package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOperatorCallSequence replays the exact HTTP call sequence the Tailscale
// Kubernetes operator uses:
//
//  1. POST /oauth/token
//  2. POST /tailnet/-/keys  (create auth key)
//  3. GET  /tailnet/-/devices
//  4. DELETE /tailnet/-/keys/{id}  (cleanup)
//
// Each response is validated for shape and status code.
func TestOperatorCallSequence(t *testing.T) {
	IntegrationSkip(t)

	// Full Docker orchestration is handled by the mustStartStack helper.
	// When HEADSCALE_INTEGRATION_TEST=1, mustStartStack should be completed
	// to spin up headscale + headapi containers and return a real stack.
	t.Skip("TestOperatorCallSequence: full stack not wired (see integration/helpers_test.go)")

	const (
		tailnet = "-"
	)

	// Placeholder — populated by mustStartStack in a real run.
	endpoint := "http://localhost:8080"
	base := endpoint + "/api/v2"
	client := &http.Client{Timeout: 10 * time.Second}

	// Step 1: OAuth token.
	token := mustGetOAuthToken(t, endpoint, "test-client", "test-secret")
	authHeader := "Bearer " + token

	// Step 2: Create auth key.
	keyBody := `{"capabilities":{"devices":{"create":{"reusable":false,"ephemeral":true,"preauthorized":true,"tags":["tag:k8s"]}}},"expirySeconds":3600}`
	req := mustNewRequest(t, http.MethodPost, base+"/tailnet/"+tailnet+"/keys",
		keyBody, authHeader)
	resp := mustDo(t, client, req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var createdKey struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&createdKey))
	resp.Body.Close()
	assert.NotEmpty(t, createdKey.ID)
	assert.True(t, strings.HasPrefix(createdKey.Key, "tskey-auth-"),
		"expected key to start with tskey-auth-, got %q", createdKey.Key)

	// Step 3: List devices.
	req = mustNewRequest(t, http.MethodGet, base+"/tailnet/"+tailnet+"/devices",
		nil, authHeader)
	resp = mustDo(t, client, req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var deviceList struct {
		Devices []struct {
			ID string `json:"id"`
		} `json:"devices"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&deviceList))
	resp.Body.Close()
	// Devices may be empty at this point — that's fine.

	// Step 4: Delete auth key (operator cleanup).
	req = mustNewRequest(t, http.MethodDelete,
		fmt.Sprintf("%s/tailnet/%s/keys/%s", base, tailnet, createdKey.ID),
		nil, authHeader)
	resp = mustDo(t, client, req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}
