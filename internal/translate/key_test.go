package translate

import (
	"testing"
	"time"

	"github.com/alam0rt/headtotails/internal/model"
	v1 "github.com/juanfont/headscale/gen/go/headscale/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPreAuthKeyToKey(t *testing.T) {
	created := timestamppb.New(time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC))
	expires := timestamppb.New(time.Date(2026, 3, 24, 0, 0, 0, 0, time.UTC))
	in := &v1.PreAuthKey{
		Id:         99,
		Used:       true,
		Reusable:   true,
		Ephemeral:  true,
		AclTags:    []string{"tag:ci"},
		CreatedAt:  created,
		Expiration: expires,
		Key:        "hskey-auth-abc",
	}

	got := PreAuthKeyToKey(in)
	assert.Equal(t, "99", got.ID)
	assert.True(t, got.Invalid)
	assert.True(t, got.Capabilities.Devices.Create.Reusable)
	assert.True(t, got.Capabilities.Devices.Create.Ephemeral)
	assert.Equal(t, []string{"tag:ci"}, got.Capabilities.Devices.Create.Tags)
	assert.Equal(t, "hskey-auth-abc", got.Key)
	assert.Equal(t, created.AsTime().Format(time.RFC3339), got.Created)
	assert.Equal(t, expires.AsTime().Format(time.RFC3339), got.Expires)
}

func TestKeyRequestToCreatePreAuthKeyRequestWithExpiry(t *testing.T) {
	req := model.CreateKeyRequest{
		Capabilities: model.KeyCapability{
			Devices: model.KeyCapabilityDevices{
				Create: model.KeyCapabilityDevicesCreate{
					Reusable:  true,
					Ephemeral: true,
					Tags:      []string{"tag:k8s"},
				},
			},
		},
		ExpirySeconds: 120,
	}

	start := time.Now()
	got := KeyRequestToCreatePreAuthKeyRequest(req, 123)
	end := time.Now()

	assert.EqualValues(t, 123, got.User)
	assert.True(t, got.Reusable)
	assert.True(t, got.Ephemeral)
	assert.Equal(t, []string{"tag:k8s"}, got.AclTags)
	require.NotNil(t, got.Expiration)
	assert.True(t, got.Expiration.AsTime().After(start.Add(119*time.Second)))
	assert.True(t, got.Expiration.AsTime().Before(end.Add(121*time.Second)))
}

func TestKeyRequestToCreatePreAuthKeyRequestWithoutExpiry(t *testing.T) {
	got := KeyRequestToCreatePreAuthKeyRequest(model.CreateKeyRequest{}, 1)
	assert.Nil(t, got.Expiration)
}
