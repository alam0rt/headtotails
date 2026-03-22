package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPolicyGetSet tests getting and setting the ACL policy.
func TestPolicyGetSet(t *testing.T) {
	IntegrationSkip(t)

	base := sharedStack.endpoint + "/api/v2"
	client := &http.Client{Timeout: 10 * time.Second}

	token := mustGetToken(t)
	authHeader := "Bearer " + token

	// Get current policy.
	req := mustNewRequest(t, http.MethodGet, base+"/tailnet/-/acl", nil, authHeader)
	resp := mustDo(t, client, req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var policy struct {
		Policy string `json:"policy"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&policy))
	resp.Body.Close()
	assert.NotEmpty(t, policy.Policy)

	// Round-trip: set the same policy back.
	req = mustNewRequest(t, http.MethodPost, base+"/tailnet/-/acl", policy.Policy, authHeader)
	resp = mustDo(t, client, req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}
