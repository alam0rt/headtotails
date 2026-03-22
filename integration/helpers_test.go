package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	dockertest "github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

// ---------------------------------------------------------------------------
// Global state shared across all tests in this package
// ---------------------------------------------------------------------------

// sharedStack is the single headscale+headapi stack for the whole test binary.
// It is initialised in TestMain and shared read-only by all tests.
var sharedStack *integrationStack

// integrationStack holds a running headscale + headapi pair.
type integrationStack struct {
	// headapi HTTP base URL e.g. "http://127.0.0.1:PORT"
	endpoint string
	// OAuth credentials configured in headapi
	oauthClientID     string
	oauthClientSecret string
	// headapi subprocess — nil when the process has already exited
	headapiCmd *exec.Cmd
	// captured log output from headapi
	headapiLog *bytes.Buffer
	headapiMu  sync.Mutex
	// headscale docker container
	pool      *dockertest.Pool
	container *dockertest.Resource
	// headscale API key issued during setup
	headscaleAPIKey string
}

// GetEndpoint returns the headapi HTTP base URL.
func (s *integrationStack) GetEndpoint() string { return s.endpoint }

// GetOAuthClientID returns the OAuth client ID.
func (s *integrationStack) GetOAuthClientID() string { return s.oauthClientID }

// GetOAuthClientSecret returns the OAuth client secret.
func (s *integrationStack) GetOAuthClientSecret() string { return s.oauthClientSecret }

// Shutdown stops headapi and removes the headscale container.
func (s *integrationStack) Shutdown() error {
	var errs []string
	if s.headapiCmd != nil && s.headapiCmd.Process != nil {
		_ = s.headapiCmd.Process.Kill()
		_ = s.headapiCmd.Wait()
	}
	if s.pool != nil && s.container != nil {
		if err := s.pool.Purge(s.container); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Logs returns the captured headapi log output (safe to call from multiple goroutines).
func (s *integrationStack) Logs() string {
	s.headapiMu.Lock()
	defer s.headapiMu.Unlock()
	return s.headapiLog.String()
}

// ---------------------------------------------------------------------------
// TestMain — package-level lifecycle
// ---------------------------------------------------------------------------

const (
	testClientID     = "headapi-test-client"
	testClientSecret = "headapi-test-secret"
	testHMACSecret   = "headapi-hmac-secret-32-chars!!" // exactly 30 chars
	testTailnetName  = "-"

	// headscale Docker image — must match the proto definitions (v0.28)
	headscaleImage = "headscale/headscale"
	headscaleTag   = "0.28"
)

func TestMain(m *testing.M) {
	// If not an integration run, just run the tests (they will self-skip).
	if os.Getenv("HEADSCALE_INTEGRATION_TEST") != "1" {
		os.Exit(m.Run())
	}

	stack, err := startStack()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to start integration stack: %v\n", err)
		os.Exit(1)
	}
	sharedStack = stack

	code := m.Run()

	// Always print headapi logs so CI has something to look at.
	fmt.Fprintf(os.Stderr, "\n=== headapi logs ===\n%s\n===================\n",
		stack.Logs())

	if err := stack.Shutdown(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: stack shutdown error: %v\n", err)
	}

	os.Exit(code)
}

// startStack brings up headscale in Docker/Podman then headapi as a local subprocess.
func startStack() (*integrationStack, error) {
	// Prefer an explicit DOCKER_HOST; fall back to the Podman user socket.
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		// Podman rootless socket (XDG_RUNTIME_DIR or /run/user/<uid>).
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir == "" {
			runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
		}
		podmanSock := runtimeDir + "/podman/podman.sock"
		if _, err := os.Stat(podmanSock); err == nil {
			dockerHost = "unix://" + podmanSock
		}
	}
	if dockerHost != "" {
		os.Setenv("DOCKER_HOST", dockerHost)
	}

	pool, err := dockertest.NewPool(dockerHost)
	if err != nil {
		return nil, fmt.Errorf("connect to Docker/Podman: %w", err)
	}
	pool.MaxWait = 2 * time.Minute

	// -----------------------------------------------------------------------
	// 1. Build headscale config
	// -----------------------------------------------------------------------
	cfgDir, err := os.MkdirTemp("", "headscale-cfg-*")
	if err != nil {
		return nil, fmt.Errorf("create temp config dir: %w", err)
	}

	headscaleCfg := `
server_url: http://127.0.0.1:18080
listen_addr: 0.0.0.0:18080
grpc_listen_addr: 0.0.0.0:50443
grpc_allow_insecure: true
noise:
  private_key_path: /var/lib/headscale/noise_private.key
private_key_path: /var/lib/headscale/private.key
prefixes:
  v4: 100.64.0.0/10
  v6: fd7a:115c:a1e0::/48
  allocation: sequential
derp:
  server:
    enabled: true
    region_id: 999
    region_code: headapi-test
    region_name: headapi test
    stun_listen_addr: 0.0.0.0:3478
    private_key_path: /var/lib/headscale/derp_private.key
  urls: []
  paths: []
  auto_update_enabled: false
  update_frequency: 24h
disable_check_updates: true
database:
  type: sqlite
  sqlite:
    path: /var/lib/headscale/db.sqlite
dns:
  magic_dns: false
  base_domain: headapi.test
  override_local_dns: false
  nameservers:
    global: []
policy:
  mode: database
`
	cfgPath := cfgDir + "/config.yaml"
	if err := os.WriteFile(cfgPath, []byte(headscaleCfg), 0644); err != nil {
		return nil, fmt.Errorf("write headscale config: %w", err)
	}

	// -----------------------------------------------------------------------
	// 2. Start headscale container
	// -----------------------------------------------------------------------
	container, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: headscaleImage,
		Tag:        headscaleTag,
		Cmd:        []string{"serve"},
		Mounts:     []string{cfgPath + ":/etc/headscale/config.yaml"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"18080/tcp": {{HostIP: "127.0.0.1", HostPort: "0"}},
			"50443/tcp": {{HostIP: "127.0.0.1", HostPort: "0"}},
		},
		ExposedPorts: []string{"18080/tcp", "50443/tcp"},
	}, func(hc *docker.HostConfig) {
		hc.AutoRemove = true
		hc.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		return nil, fmt.Errorf("start headscale container: %w", err)
	}

	grpcPort := container.GetPort("50443/tcp")
	httpPort := container.GetPort("18080/tcp")
	fmt.Printf("[stack] headscale gRPC=127.0.0.1:%s HTTP=127.0.0.1:%s\n",
		grpcPort, httpPort)

	// Wait for headscale HTTP to be ready.
	headscaleHTTP := "http://127.0.0.1:" + httpPort
	if err := pool.Retry(func() error {
		resp, err := http.Get(headscaleHTTP + "/health") //nolint:gosec
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("headscale /health returned %d", resp.StatusCode)
		}
		return nil
	}); err != nil {
		_ = pool.Purge(container)
		return nil, fmt.Errorf("wait for headscale: %w", err)
	}
	fmt.Println("[stack] headscale is healthy")

	// -----------------------------------------------------------------------
	// 3. Bootstrap: create user + API key via docker exec
	// -----------------------------------------------------------------------
	containerID := container.Container.ID

	// Create user — ignore "already exists" errors (idempotent).
	if out, err := dockerExecOutput(containerID, "headscale", "users", "create", "testuser"); err != nil {
		if !strings.Contains(out, "already") && !strings.Contains(out, "UNIQUE") {
			_ = pool.Purge(container)
			return nil, fmt.Errorf("create headscale user: %w\n%s", err, out)
		}
	}
	fmt.Println("[stack] headscale user 'testuser' created")

	// Get the user ID (headscale 0.28 requires numeric ID for preauthkeys).
	userListOut, err := dockerExecOutput(containerID, "headscale", "users", "list", "--output", "json")
	if err != nil {
		_ = pool.Purge(container)
		return nil, fmt.Errorf("list headscale users: %w", err)
	}
	var users []struct {
		ID int `json:"id"`
	}
	if jsonErr := json.Unmarshal([]byte(userListOut), &users); jsonErr != nil || len(users) == 0 {
		_ = pool.Purge(container)
		return nil, fmt.Errorf("parse user list: %v\nraw: %s", jsonErr, userListOut)
	}
	userIDStr := fmt.Sprintf("%d", users[0].ID)

	// Create a preauthkey so headapi's CreatePreAuthKey has a user to bind to.
	_ = userIDStr // used below if needed; headapi calls ListUsers on its own

	apiKeyOut, err := dockerExecOutput(containerID, "headscale", "apikeys", "create", "--expiration", "24h")
	if err != nil {
		_ = pool.Purge(container)
		return nil, fmt.Errorf("create headscale API key: %w", err)
	}
	apiKey := strings.TrimSpace(apiKeyOut)
	fmt.Printf("[stack] headscale API key: %s\n", apiKey)

	// -----------------------------------------------------------------------
	// 4. Build headapi binary
	// -----------------------------------------------------------------------
	binaryPath, err := buildHeadapi()
	if err != nil {
		_ = pool.Purge(container)
		return nil, fmt.Errorf("build headapi: %w", err)
	}
	fmt.Printf("[stack] headapi binary: %s\n", binaryPath)

	// -----------------------------------------------------------------------
	// 5. Start headapi subprocess on a free port
	// -----------------------------------------------------------------------
	headapiPort, err := freePort()
	if err != nil {
		_ = pool.Purge(container)
		return nil, fmt.Errorf("find free port: %w", err)
	}

	logBuf := &bytes.Buffer{}
	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(),
		"LISTEN_ADDR=:"+headapiPort,
		"HEADSCALE_ADDR=127.0.0.1:"+grpcPort,
		"HEADSCALE_API_KEY="+apiKey,
		"TAILNET_NAME="+testTailnetName,
		"OAUTH_CLIENT_ID="+testClientID,
		"OAUTH_CLIENT_SECRET="+testClientSecret,
		"OAUTH_HMAC_SECRET="+testHMACSecret,
	)
	cmd.Stdout = logBuf
	cmd.Stderr = logBuf

	if err := cmd.Start(); err != nil {
		_ = pool.Purge(container)
		return nil, fmt.Errorf("start headapi: %w", err)
	}
	fmt.Printf("[stack] headapi PID=%d listening on :%s\n", cmd.Process.Pid, headapiPort)

	headapiEndpoint := "http://127.0.0.1:" + headapiPort

	// Wait for headapi to be healthy.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(headapiEndpoint + "/healthz") //nolint:gosec
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		// Check if the process already died.
		if cmd.ProcessState != nil {
			_ = pool.Purge(container)
			return nil, fmt.Errorf("headapi exited prematurely: %s", logBuf.String())
		}
		time.Sleep(250 * time.Millisecond)
	}
	if time.Now().After(deadline) {
		_ = cmd.Process.Kill()
		_ = pool.Purge(container)
		return nil, fmt.Errorf("headapi did not become healthy: %s", logBuf.String())
	}
	fmt.Println("[stack] headapi is healthy")

	return &integrationStack{
		endpoint:          headapiEndpoint,
		oauthClientID:     testClientID,
		oauthClientSecret: testClientSecret,
		headapiCmd:        cmd,
		headapiLog:        logBuf,
		pool:              pool,
		container:         container,
		headscaleAPIKey:   apiKey,
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers used by TestMain
// ---------------------------------------------------------------------------

// buildHeadapi compiles the headapi binary and returns its path.
func buildHeadapi() (string, error) {
	tmp, err := os.CreateTemp("", "headapi-integration-*")
	if err != nil {
		return "", err
	}
	tmp.Close()
	binaryPath := tmp.Name()

	// Locate the module root (parent of the integration package).
	moduleRoot, err := findModuleRoot()
	if err != nil {
		return "", err
	}

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/headapi")
	cmd.Dir = moduleRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go build failed: %w\n%s", err, out)
	}
	return binaryPath, nil
}

// findModuleRoot walks up from the integration directory to find go.mod.
func findModuleRoot() (string, error) {
	// This file is in <module-root>/integration/
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir, nil
		}
		parent := dir[:strings.LastIndex(dir, "/")]
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// freePort returns a free TCP port on localhost.
func freePort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	return port, err
}

// dockerExec runs a command inside a container, discarding output.
func dockerExec(containerID string, args ...string) error {
	_, err := dockerExecOutput(containerID, args...)
	return err
}

// dockerExecOutput runs a command inside a container and returns stdout+stderr.
func dockerExecOutput(containerID string, args ...string) (string, error) {
	fullArgs := append([]string{"exec", containerID}, args...)
	out, err := exec.Command("docker", fullArgs...).CombinedOutput()
	return string(out), err
}

// ---------------------------------------------------------------------------
// Per-test helpers
// ---------------------------------------------------------------------------

// IntegrationSkip skips the test unless HEADSCALE_INTEGRATION_TEST=1 is set.
func IntegrationSkip(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if os.Getenv("HEADSCALE_INTEGRATION_TEST") != "1" {
		t.Skip("skipping integration test; set HEADSCALE_INTEGRATION_TEST=1 to run")
	}
}

// mustGetToken fetches an OAuth bearer token from the shared stack.
func mustGetToken(t *testing.T) string {
	t.Helper()
	s := sharedStack
	resp, err := http.PostForm(s.endpoint+"/oauth/token", map[string][]string{
		"grant_type":    {"client_credentials"},
		"client_id":     {s.oauthClientID},
		"client_secret": {s.oauthClientSecret},
	})
	if err != nil {
		t.Fatalf("POST /oauth/token: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /oauth/token status %d: %s", resp.StatusCode, body)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	return tok.AccessToken
}

// mustGetOAuthToken fetches an OAuth bearer token from the specified endpoint.
// Kept as a package-level function for tests that pass custom endpoints.
func mustGetOAuthToken(t *testing.T, endpoint, clientID, clientSecret string) string {
	t.Helper()
	resp, err := http.PostForm(endpoint+"/oauth/token", map[string][]string{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	})
	if err != nil {
		t.Fatalf("POST %s/oauth/token: %v", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /oauth/token status %d: %s", resp.StatusCode, body)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	return tok.AccessToken
}

// mustNewRequest creates an HTTP request with an optional Bearer auth header.
func mustNewRequest(t *testing.T, method, url string, body interface{}, authHeader string) *http.Request {
	t.Helper()
	var bodyStr string
	if body != nil {
		switch s := body.(type) {
		case string:
			bodyStr = s
		default:
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal request body: %v", err)
			}
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
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, url, err)
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	return req
}

// mustDo executes an HTTP request and returns the response, failing on error.
func mustDo(t *testing.T, client *http.Client, req *http.Request) *http.Response {
	t.Helper()
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP %s %s: %v", req.Method, req.URL, err)
	}
	return resp
}

// formatID formats a numeric ID as a string.
func formatID(id uint64) string {
	return fmt.Sprintf("%d", id)
}

// headapiStack is the type used by mustStartStack callers (oauth_test.go).
type headapiStack struct {
	ha interface {
		GetEndpoint() string
		GetOAuthClientID() string
		GetOAuthClientSecret() string
		Shutdown() error
	}
	cleanup func()
}

// mustStartStack returns the global shared stack (already started in TestMain).
func mustStartStack(t *testing.T, _ *dockertest.Pool, _ string) headapiStack {
	t.Helper()
	IntegrationSkip(t)
	if sharedStack == nil {
		t.Fatal("sharedStack is nil — TestMain did not start the stack")
	}
	return headapiStack{
		ha:      sharedStack,
		cleanup: func() {},
	}
}

// mustRequireNoErr is a tiny helper to avoid importing testify in this file
// when we only need a fatal-on-error check.
func mustRequireNoErr(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

// stackEndpoint returns the base URL + /api/v2 for the shared stack.
func stackEndpoint() (base string, client *http.Client) {
	return sharedStack.endpoint + "/api/v2",
		&http.Client{Timeout: 10 * time.Second}
}

// stackAuth returns a Bearer auth header for a fresh OAuth token.
func stackAuth(t *testing.T) string {
	t.Helper()
	return "Bearer " + mustGetToken(t)
}

// retryCtx polls fn until it returns nil or the context is done.
func retryCtx(ctx context.Context, interval time.Duration, fn func() error) error {
	for {
		if err := fn(); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

