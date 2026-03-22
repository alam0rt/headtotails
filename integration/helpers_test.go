package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// sharedStack is the single headscale+headtotails stack for the whole test binary.
// It is initialised in TestMain and shared read-only by all tests.
var sharedStack *integrationStack

// integrationStack holds a running headscale + headtotails pair.
type integrationStack struct {
	// headtotails HTTP base URL e.g. "http://127.0.0.1:PORT"
	endpoint string
	// OAuth credentials configured in headtotails
	oauthClientID     string
	oauthClientSecret string
	// headtotails subprocess — nil when the process has already exited
	headtotailsCmd *exec.Cmd
	// captured log output from headtotails
	headtotailsLog *bytes.Buffer
	headtotailsMu  sync.Mutex
	// headscale docker container
	pool      *dockertest.Pool
	container *dockertest.Resource
	// headscale API key issued during setup
	headscaleAPIKey string
}

// GetEndpoint returns the headtotails HTTP base URL.
func (s *integrationStack) GetEndpoint() string { return s.endpoint }

// GetOAuthClientID returns the OAuth client ID.
func (s *integrationStack) GetOAuthClientID() string { return s.oauthClientID }

// GetOAuthClientSecret returns the OAuth client secret.
func (s *integrationStack) GetOAuthClientSecret() string { return s.oauthClientSecret }

// Shutdown stops headtotails and removes the headscale container.
func (s *integrationStack) Shutdown() error {
	var errs []string
	if s.headtotailsCmd != nil && s.headtotailsCmd.Process != nil {
		_ = s.headtotailsCmd.Process.Kill()
		_ = s.headtotailsCmd.Wait()
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

// Logs returns the captured headtotails log output (safe to call from multiple goroutines).
func (s *integrationStack) Logs() string {
	s.headtotailsMu.Lock()
	defer s.headtotailsMu.Unlock()
	return s.headtotailsLog.String()
}

// ---------------------------------------------------------------------------
// TestMain — package-level lifecycle
// ---------------------------------------------------------------------------

const (
	testClientID     = "headtotails-test-client"
	testClientSecret = "headtotails-test-secret"
	testHMACSecret   = "headtotails-hmac-secret-32-chars!!" // exactly 30 chars
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

	// Always print headtotails logs so CI has something to look at.
	fmt.Fprintf(os.Stderr, "\n=== headtotails logs ===\n%s\n===================\n",
		stack.Logs())

	if err := stack.Shutdown(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: stack shutdown error: %v\n", err)
	}

	os.Exit(code)
}

// startStack brings up headscale in Docker/Podman then headtotails as a local subprocess.
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
		if err := os.Setenv("DOCKER_HOST", dockerHost); err != nil {
			return nil, fmt.Errorf("set DOCKER_HOST: %w", err)
		}
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
    region_code: headtotails-test
    region_name: headtotails test
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
  base_domain: headtotails.test
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
		if err := resp.Body.Close(); err != nil {
			return err
		}
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

	// Create a preauthkey so headtotails's CreatePreAuthKey has a user to bind to.
	_ = userIDStr // used below if needed; headtotails calls ListUsers on its own

	apiKeyOut, err := dockerExecOutput(containerID, "headscale", "apikeys", "create", "--expiration", "24h")
	if err != nil {
		_ = pool.Purge(container)
		return nil, fmt.Errorf("create headscale API key: %w", err)
	}
	apiKey := strings.TrimSpace(apiKeyOut)
	fmt.Printf("[stack] headscale API key: %s\n", apiKey)

	// -----------------------------------------------------------------------
	// 4. Build headtotails binary
	// -----------------------------------------------------------------------
	binaryPath, err := buildHeadtotails()
	if err != nil {
		_ = pool.Purge(container)
		return nil, fmt.Errorf("build headtotails: %w", err)
	}
	fmt.Printf("[stack] headtotails binary: %s\n", binaryPath)

	// -----------------------------------------------------------------------
	// 5. Start headtotails subprocess on a free port
	// -----------------------------------------------------------------------
	headtotailsPort, err := freePort()
	if err != nil {
		_ = pool.Purge(container)
		return nil, fmt.Errorf("find free port: %w", err)
	}

	logBuf := &bytes.Buffer{}
	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(),
		"LISTEN_ADDR=:"+headtotailsPort,
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
		return nil, fmt.Errorf("start headtotails: %w", err)
	}
	fmt.Printf("[stack] headtotails PID=%d listening on :%s\n", cmd.Process.Pid, headtotailsPort)

	headtotailsEndpoint := "http://127.0.0.1:" + headtotailsPort

	// Wait for headtotails to be healthy.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(headtotailsEndpoint + "/healthz") //nolint:gosec
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			break
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		// Check if the process already died.
		if cmd.ProcessState != nil {
			_ = pool.Purge(container)
			return nil, fmt.Errorf("headtotails exited prematurely: %s", logBuf.String())
		}
		time.Sleep(250 * time.Millisecond)
	}
	if time.Now().After(deadline) {
		_ = cmd.Process.Kill()
		_ = pool.Purge(container)
		return nil, fmt.Errorf("headtotails did not become healthy: %s", logBuf.String())
	}
	fmt.Println("[stack] headtotails is healthy")

	return &integrationStack{
		endpoint:          headtotailsEndpoint,
		oauthClientID:     testClientID,
		oauthClientSecret: testClientSecret,
		headtotailsCmd:    cmd,
		headtotailsLog:    logBuf,
		pool:              pool,
		container:         container,
		headscaleAPIKey:   apiKey,
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers used by TestMain
// ---------------------------------------------------------------------------

// buildHeadtotails compiles the headtotails binary and returns its path.
func buildHeadtotails() (string, error) {
	tmp, err := os.CreateTemp("", "headtotails-integration-*")
	if err != nil {
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}
	binaryPath := tmp.Name()

	// Locate the module root (parent of the integration package).
	moduleRoot, err := findModuleRoot()
	if err != nil {
		return "", err
	}

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/headtotails")
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
	_, port, err := net.SplitHostPort(l.Addr().String())
	closeErr := l.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	return port, nil
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

// headtotailsStack is the type used by mustStartStack callers (oauth_test.go).
type headtotailsStack struct {
	ha interface {
		GetEndpoint() string
		GetOAuthClientID() string
		GetOAuthClientSecret() string
		Shutdown() error
	}
	cleanup func()
}

// mustStartStack returns the global shared stack (already started in TestMain).
func mustStartStack(t *testing.T, _ *dockertest.Pool, _ string) headtotailsStack {
	t.Helper()
	IntegrationSkip(t)
	if sharedStack == nil {
		t.Fatal("sharedStack is nil — TestMain did not start the stack")
	}
	return headtotailsStack{
		ha:      sharedStack,
		cleanup: func() {},
	}
}
