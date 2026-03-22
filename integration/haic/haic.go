// Package haic provides HeadapiInContainer, which runs the headtotails binary in a
// Docker container on the same network as a headscale container.
// It mirrors the hsic (HeadscaleInContainer) pattern from headscale's own
// integration test suite.
package haic

import (
	"fmt"
	"net/http"
	"time"

	dockertest "github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

const (
	headtotailsImage = "headtotails"
	headtotailsPort  = "8080/tcp"
)

// ControlServer is the interface that HeadscaleInContainer implements.
// We depend on this interface to avoid importing headscale's integration package.
type ControlServer interface {
	GetHostname() string
	GetEndpoint() string
}

// HeadapiInContainer runs the headtotails binary in a Docker container.
type HeadapiInContainer struct {
	pool      *dockertest.Pool
	container *dockertest.Resource
	networks  []*dockertest.Network

	headscaleAddr     string
	headscaleAPIKey   string
	oauthClientID     string
	oauthClientSecret string
	hmacSecret        string
	tailnetName       string
	listenPort        string
}

// Option is a functional option for HeadapiInContainer.
type Option func(*HeadapiInContainer)

// WithOAuthCredentials sets the OAuth client ID and secret.
func WithOAuthCredentials(id, secret string) Option {
	return func(h *HeadapiInContainer) {
		h.oauthClientID = id
		h.oauthClientSecret = secret
	}
}

// WithHeadscaleAPIKey sets the headscale API key headtotails will use.
func WithHeadscaleAPIKey(key string) Option {
	return func(h *HeadapiInContainer) {
		h.headscaleAPIKey = key
	}
}

// WithTailnetName sets the tailnet name (default: "-").
func WithTailnetName(name string) Option {
	return func(h *HeadapiInContainer) {
		h.tailnetName = name
	}
}

// WithHMACSecret sets the HMAC secret used for OAuth token signing.
func WithHMACSecret(secret string) Option {
	return func(h *HeadapiInContainer) {
		h.hmacSecret = secret
	}
}

// New builds headtotails from source using a two-stage Docker build and starts it
// on the given Docker network, pointing at the provided headscale instance.
func New(
	pool *dockertest.Pool,
	networks []*dockertest.Network,
	headscale ControlServer,
	opts ...Option,
) (*HeadapiInContainer, error) {
	h := &HeadapiInContainer{
		pool:              pool,
		networks:          networks,
		oauthClientID:     "headtotails-client",
		oauthClientSecret: "headtotails-secret",
		hmacSecret:        "headtotails-hmac-secret-32-chars!!!",
		tailnetName:       "-",
		// headscale gRPC is typically on port 50443 internally.
		headscaleAddr: headscale.GetHostname() + ":50443",
	}

	for _, o := range opts {
		o(h)
	}

	// Build the Docker image from the workspace root.
	buildOpts := docker.BuildImageOptions{
		Name:           headtotailsImage,
		ContextDir:     "../../", // relative to integration/haic — adjust as needed
		Dockerfile:     "Dockerfile",
		SuppressOutput: true,
		Pull:           false,
	}
	if err := pool.Client.BuildImage(buildOpts); err != nil {
		return nil, fmt.Errorf("build headtotails image: %w", err)
	}

	networkIDs := make([]string, len(networks))
	for i, n := range networks {
		networkIDs[i] = n.Network.ID
	}

	env := []string{
		"LISTEN_ADDR=:8080",
		"HEADSCALE_ADDR=" + h.headscaleAddr,
		"HEADSCALE_API_KEY=" + h.headscaleAPIKey,
		"TAILNET_NAME=" + h.tailnetName,
		"OAUTH_CLIENT_ID=" + h.oauthClientID,
		"OAUTH_CLIENT_SECRET=" + h.oauthClientSecret,
		"OAUTH_HMAC_SECRET=" + h.hmacSecret,
	}

	container, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: headtotailsImage,
		Tag:        "latest",
		Env:        env,
		NetworkID:  networkIDs[0],
	}, func(hc *docker.HostConfig) {
		hc.AutoRemove = true
		hc.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		return nil, fmt.Errorf("start headtotails container: %w", err)
	}

	h.container = container
	h.listenPort = container.GetPort(headtotailsPort)

	return h, nil
}

// GetEndpoint returns the HTTP endpoint for the headtotails container (from the host).
func (h *HeadapiInContainer) GetEndpoint() string {
	return fmt.Sprintf("http://localhost:%s", h.listenPort)
}

// GetOAuthClientID returns the configured OAuth client ID.
func (h *HeadapiInContainer) GetOAuthClientID() string { return h.oauthClientID }

// GetOAuthClientSecret returns the configured OAuth client secret.
func (h *HeadapiInContainer) GetOAuthClientSecret() string { return h.oauthClientSecret }

// WaitForRunning polls /healthz until headtotails is up or timeout expires.
func (h *HeadapiInContainer) WaitForRunning() error {
	healthURL := h.GetEndpoint() + "/healthz"
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL) //nolint:gosec
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("headtotails did not become healthy within timeout")
}

// Shutdown stops and removes the headtotails container.
func (h *HeadapiInContainer) Shutdown() error {
	if h.container != nil {
		return h.pool.Purge(h.container)
	}
	return nil
}
