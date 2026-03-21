package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserList tests listing users via the headapi.
func TestUserList(t *testing.T) {
	IntegrationSkip(t)
	t.Skip("TestUserList: full Docker stack not wired")

	endpoint := "http://localhost:8080"
	base := endpoint + "/api/v2"
	client := &http.Client{Timeout: 10 * time.Second}

	token := mustGetOAuthToken(t, endpoint, "test-client", "test-secret")
	authHeader := "Bearer " + token

	req := mustNewRequest(t, http.MethodGet, base+"/tailnet/-/users", nil, authHeader)
	resp := mustDo(t, client, req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var list struct {
		Users []struct {
			ID        string `json:"id"`
			LoginName string `json:"loginName"`
		} `json:"users"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
	resp.Body.Close()

	// Depending on headscale setup, there may be zero or more users.
	assert.NotNil(t, list.Users)
}
