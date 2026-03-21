package integration

import (
	"os"
	"testing"
)

// IntegrationSkip skips the test unless HEADSCALE_INTEGRATION_TEST=1 is set.
// This mirrors headscale's own integration test skip pattern.
func IntegrationSkip(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if os.Getenv("HEADSCALE_INTEGRATION_TEST") != "1" {
		t.Skip("skipping integration test; set HEADSCALE_INTEGRATION_TEST=1 to run")
	}
}

