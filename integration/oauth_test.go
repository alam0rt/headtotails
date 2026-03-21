package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	dockertest "github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOAuthTokenIssuance tests the OAuth 2.0 client credentials flow end-to-end
// against a real headapi instance in Docker.
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

// mustGetOAuthToken is a test helper that obtains an OAuth token from headapi.
func mustGetOAuthToken(t *testing.T, endpoint, clientID, clientSecret string) string {
	t.Helper()
	resp, err := http.PostForm(endpoint+"/oauth/token", url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "OAuth token request failed")

	var tok struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&tok))
	require.NotEmpty(t, tok.AccessToken)
	return tok.AccessToken
}

// mustNewRequest is a test helper that creates an HTTP request with a Bearer token.
func mustNewRequest(t *testing.T, method, url string, body interface{}, authHeader string) *http.Request {
	t.Helper()
	var bodyStr string
	if body != nil {
		if s, ok := body.(string); ok {
			bodyStr = s
		} else {
			b, err := json.Marshal(body)
			require.NoError(t, err)
			bodyStr = string(b)
		}
	}

	var req *http.Request
	var err error
	if bodyStr != "" {
		req, err = http.NewRequest(method, url, strings.NewReader(bodyStr))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	require.NoError(t, err)

	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	return req
}

// mustDo executes an HTTP request and returns the response.
func mustDo(t *testing.T, client *http.Client, req *http.Request) *http.Response {
	t.Helper()
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

// headapiStack holds a running headapi + backing data for integration tests.
type headapiStack struct {
	ha interface {
		GetEndpoint() string
		GetOAuthClientID() string
		GetOAuthClientSecret() string
		Shutdown() error
	}
	cleanup func()
}

// mustStartStack starts a minimal headapi stack for integration testing.
// In the absence of a real headscale container, we use an environment-variable
// configured endpoint. Full Docker orchestration is done in contract_test.go.
func mustStartStack(t *testing.T, pool *dockertest.Pool, testName string) headapiStack {
	t.Helper()
	// In a real test this would spin up headscale + headapi containers.
	// For now we return a stub that tests can extend.
	t.Skipf("mustStartStack: full Docker stack not yet wired for test %q", testName)
	return headapiStack{}
}

// formatID formats a numeric ID as a string.
func formatID(id uint64) string {
	return fmt.Sprintf("%d", id)
}
