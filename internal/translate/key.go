package translate

import (
	"fmt"
	"time"

	"github.com/alam0rt/headtotails/internal/model"
	v1 "github.com/juanfont/headscale/gen/go/headscale/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PreAuthKeyToKey converts a headscale PreAuthKey to a Tailscale API Key.
func PreAuthKeyToKey(k *v1.PreAuthKey) model.Key {
	key := model.Key{
		ID:      fmt.Sprintf("%d", k.GetId()),
		Invalid: k.GetUsed(),
		Capabilities: model.KeyCapability{
			Devices: model.KeyCapabilityDevices{
				Create: model.KeyCapabilityDevicesCreate{
					Reusable:  k.GetReusable(),
					Ephemeral: k.GetEphemeral(),
					Tags:      k.GetAclTags(),
				},
			},
		},
	}

	if ca := k.GetCreatedAt(); ca != nil {
		key.Created = ca.AsTime().Format(time.RFC3339)
	}

	if exp := k.GetExpiration(); exp != nil {
		key.Expires = exp.AsTime().Format(time.RFC3339)
	}

	// The actual key value is only returned on creation by headscale,
	// but include it in translation if present.
	// headscale >=0.28 already returns "hskey-auth-..." so we just use
	// it as-is without adding a second prefix.
	if k.GetKey() != "" {
		key.Key = k.GetKey()
	}

	return key
}

// KeyRequestToCreatePreAuthKeyRequest converts a Tailscale CreateKeyRequest to a headscale proto request.
// userID is the headscale user ID to scope the key to.
func KeyRequestToCreatePreAuthKeyRequest(r model.CreateKeyRequest, userID uint64) *v1.CreatePreAuthKeyRequest {
	req := &v1.CreatePreAuthKeyRequest{
		User:      userID,
		Reusable:  r.Capabilities.Devices.Create.Reusable,
		Ephemeral: r.Capabilities.Devices.Create.Ephemeral,
		AclTags:   r.Capabilities.Devices.Create.Tags,
	}

	// Default to 1 hour if no expiry is specified. headscale stores a zero
	// time when Expiration is nil, which it then treats as already-expired
	// when validating the key during node registration.
	expirySeconds := r.ExpirySeconds
	if expirySeconds <= 0 {
		expirySeconds = 3600 // 1 hour
	}
	expiry := time.Now().Add(time.Duration(expirySeconds) * time.Second)
	req.Expiration = timestamppb.New(expiry)

	return req
}
