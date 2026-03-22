package integration

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tailscale "tailscale.com/client/tailscale/v2"
)

// newTailscaleClient returns a *tailscale.Client pointed at the shared
// headtotails instance, authenticating via the official OAuth flow.
func newTailscaleClient(t *testing.T) *tailscale.Client {
	t.Helper()
	IntegrationSkip(t)

	base, err := url.Parse(sharedStack.endpoint)
	require.NoError(t, err)

	return &tailscale.Client{
		BaseURL: base,
		Tailnet: "-",
		Auth: &tailscale.OAuth{
			ClientID:     sharedStack.oauthClientID,
			ClientSecret: sharedStack.oauthClientSecret,
			Scopes:       []string{"all:write"},
		},
	}
}

// TestClientKeys exercises the full auth-key lifecycle through the official
// Tailscale client: create, list, get, delete.
func TestClientKeys(t *testing.T) {
	client := newTailscaleClient(t)
	ctx := context.Background()

	created, err := client.Keys().CreateAuthKey(ctx, tailscale.CreateKeyRequest{
		Capabilities: tailscale.KeyCapabilities{
			Devices: struct {
				Create struct {
					Reusable      bool     `json:"reusable"`
					Ephemeral     bool     `json:"ephemeral"`
					Tags          []string `json:"tags"`
					Preauthorized bool     `json:"preauthorized"`
				} `json:"create"`
			}{
				Create: struct {
					Reusable      bool     `json:"reusable"`
					Ephemeral     bool     `json:"ephemeral"`
					Tags          []string `json:"tags"`
					Preauthorized bool     `json:"preauthorized"`
				}{
					Reusable:      false,
					Ephemeral:     true,
					Preauthorized: true,
				},
			},
		},
		ExpirySeconds: 3600,
	})
	require.NoError(t, err, "CreateAuthKey")
	assert.NotEmpty(t, created.ID, "key ID should be set")
	assert.NotEmpty(t, created.Key, "key secret should be returned on create")

	keys, err := client.Keys().List(ctx, false)
	require.NoError(t, err, "List keys")

	var found bool
	for _, k := range keys {
		if k.ID == created.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "created key %q should appear in list", created.ID)

	got, err := client.Keys().Get(ctx, created.ID)
	require.NoError(t, err, "Get key")
	assert.Equal(t, created.ID, got.ID)

	err = client.Keys().Delete(ctx, created.ID)
	require.NoError(t, err, "Delete key")
}

// TestClientDevices lists devices through the official client and, if any
// exist, fetches one by ID.
func TestClientDevices(t *testing.T) {
	client := newTailscaleClient(t)
	ctx := context.Background()

	devices, err := client.Devices().List(ctx)
	require.NoError(t, err, "List devices")

	if len(devices) > 0 {
		dev, err := client.Devices().Get(ctx, devices[0].ID)
		require.NoError(t, err, "Get device")
		assert.Equal(t, devices[0].ID, dev.ID)
	}
}

// TestClientUsers lists users and asserts the "testuser" created by TestMain
// is present.
func TestClientUsers(t *testing.T) {
	client := newTailscaleClient(t)
	ctx := context.Background()

	users, err := client.Users().List(ctx, nil, nil)
	require.NoError(t, err, "List users")
	assert.NotEmpty(t, users, "expected at least one user (testuser)")
}

// TestClientPolicyFile round-trips the ACL policy: get then set.
func TestClientPolicyFile(t *testing.T) {
	client := newTailscaleClient(t)
	ctx := context.Background()

	acl, err := client.PolicyFile().Get(ctx)
	require.NoError(t, err, "Get policy")
	require.NotNil(t, acl, "ACL should not be nil")

	err = client.PolicyFile().Set(ctx, *acl, "")
	require.NoError(t, err, "Set policy (round-trip)")
}

// TestClientOperatorSequence replays the exact call sequence the Tailscale
// Kubernetes operator uses, driven entirely by the official client library:
//
//  1. Create auth key
//  2. List devices
//  3. Delete auth key (cleanup)
func TestClientOperatorSequence(t *testing.T) {
	client := newTailscaleClient(t)
	ctx := context.Background()

	// 1. Create auth key.
	key, err := client.Keys().CreateAuthKey(ctx, tailscale.CreateKeyRequest{
		Capabilities: tailscale.KeyCapabilities{
			Devices: struct {
				Create struct {
					Reusable      bool     `json:"reusable"`
					Ephemeral     bool     `json:"ephemeral"`
					Tags          []string `json:"tags"`
					Preauthorized bool     `json:"preauthorized"`
				} `json:"create"`
			}{
				Create: struct {
					Reusable      bool     `json:"reusable"`
					Ephemeral     bool     `json:"ephemeral"`
					Tags          []string `json:"tags"`
					Preauthorized bool     `json:"preauthorized"`
				}{
					Reusable:      false,
					Ephemeral:     true,
					Preauthorized: true,
				},
			},
		},
		ExpirySeconds: 3600,
	})
	require.NoError(t, err, "create auth key")
	require.NotEmpty(t, key.ID)
	require.NotEmpty(t, key.Key)

	// 2. List devices (may be empty — that's OK).
	_, err = client.Devices().List(ctx)
	require.NoError(t, err, "list devices")

	// 3. Delete auth key (operator cleanup).
	err = client.Keys().Delete(ctx, key.ID)
	require.NoError(t, err, "delete auth key")
}
