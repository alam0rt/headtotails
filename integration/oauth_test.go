package integration

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	dockertest "github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOAuthTokenIssuance tests the OAuth 2.0 client credentials flow end-to-end
// against a real headapi instance.
func TestOAuthTokenIssuance(t *testing.T) {
	IntegrationSkip(t)

	pool, err := dockertest.NewPool("")
	require.NoError(t, err)
	pool.MaxWait = 2 * time.Minute

	stack := mustStartStack(t, pool, "oauth")
	defer stack.cleanup()

	endpoint := stack.ha.GetEndpoint()

	t.Run("ValidCredentials", func(t *testing.T) {
		resp, err := http.PostForm(endpoint+"/oauth/token", url.Values{
			"grant_type":    {"client_credentials"},
			"client_id":     {stack.ha.GetOAuthClientID()},
			"client_secret": {stack.ha.GetOAuthClientSecret()},
		})
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var tok struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
			ExpiresIn   int    `json:"expires_in"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&tok))
		assert.NotEmpty(t, tok.AccessToken)
		assert.Equal(t, "Bearer", tok.TokenType)
		assert.Greater(t, tok.ExpiresIn, 0)
	})

	t.Run("WrongSecret", func(t *testing.T) {
		resp, err := http.PostForm(endpoint+"/oauth/token", url.Values{
			"grant_type":    {"client_credentials"},
			"client_id":     {stack.ha.GetOAuthClientID()},
			"client_secret": {"wrong-secret"},
		})
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("MissingGrantType", func(t *testing.T) {
		resp, err := http.PostForm(endpoint+"/oauth/token", url.Values{
			"client_id":     {stack.ha.GetOAuthClientID()},
			"client_secret": {stack.ha.GetOAuthClientSecret()},
		})
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
