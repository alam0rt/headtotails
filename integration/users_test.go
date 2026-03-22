package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserList tests listing users via headapi.
func TestUserList(t *testing.T) {
	IntegrationSkip(t)

	base := sharedStack.endpoint + "/api/v2"
	client := &http.Client{Timeout: 10 * time.Second}

	token := mustGetToken(t)
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

	// TestMain creates "testuser" — it must appear.
	assert.NotEmpty(t, list.Users, "expected at least one user (testuser)")
}
