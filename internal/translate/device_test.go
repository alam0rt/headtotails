package translate

import (
	"testing"
	"time"

	v1 "github.com/juanfont/headscale/gen/go/headscale/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNodeToDevice(t *testing.T) {
	lastSeen := timestamppb.New(time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC))
	expiry := timestamppb.New(time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC))
	node := &v1.Node{
		Id:          42,
		Name:        "node-42",
		IpAddresses: []string{"100.64.0.42"},
		Tags:        []string{"tag:prod"},
		NodeKey:     "node-key",
		User:        &v1.User{Name: "alice"},
		LastSeen:    lastSeen,
		Expiry:      expiry,
		Online:      false,
	}

	got := NodeToDevice(node)
	assert.Equal(t, "42", got.ID)
	assert.Equal(t, "node-42", got.Name)
	assert.Equal(t, "node-42", got.Hostname)
	assert.Equal(t, []string{"100.64.0.42"}, got.Addresses)
	assert.Equal(t, []string{"tag:prod"}, got.Tags)
	assert.Equal(t, "node-key", got.NodeKey)
	assert.Equal(t, "alice", got.User)
	assert.Equal(t, lastSeen.AsTime().Format(time.RFC3339), got.LastSeen)
	assert.Equal(t, expiry.AsTime().Format(time.RFC3339), got.Expires)
	assert.False(t, got.KeyExpiryDisabled)
	assert.True(t, got.Authorized)
}

func TestNodeToDeviceNoExpiryDisablesKeyExpiry(t *testing.T) {
	got := NodeToDevice(&v1.Node{Id: 7, Name: "node-7"})
	assert.True(t, got.KeyExpiryDisabled)
}

func TestNodeToDeviceRoutes(t *testing.T) {
	node := &v1.Node{
		AvailableRoutes: []string{"10.0.0.0/8", "192.168.0.0/16"},
		ApprovedRoutes:  []string{"10.0.0.0/8"},
	}

	got := NodeToDeviceRoutes(node)
	assert.Equal(t, []string{"10.0.0.0/8", "192.168.0.0/16"}, got.AdvertisedRoutes)
	assert.Equal(t, []string{"10.0.0.0/8"}, got.EnabledRoutes)
}
