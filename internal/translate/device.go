package translate

import (
	"fmt"
	"time"

	"github.com/alam0rt/headtotails/internal/model"
	v1 "github.com/juanfont/headscale/gen/go/headscale/v1"
)

// NodeToDevice converts a headscale proto Node to a Tailscale API Device.
func NodeToDevice(n *v1.Node) model.Device {
	d := model.Device{
		ID:         fmt.Sprintf("%d", n.GetId()),
		Name:       n.GetName(),
		Hostname:   n.GetName(),
		Addresses:  n.GetIpAddresses(),
		Authorized: true,
		Tags:       n.GetTags(),
		NodeKey:    n.GetNodeKey(),
	}

	if u := n.GetUser(); u != nil {
		d.User = u.GetName()
	}

	if ls := n.GetLastSeen(); ls != nil {
		d.LastSeen = ls.AsTime().Format(time.RFC3339)
	}

	if exp := n.GetExpiry(); exp != nil {
		t := exp.AsTime()
		d.KeyExpiryDisabled = t.IsZero()
		d.Expires = t.Format(time.RFC3339)
	} else {
		d.KeyExpiryDisabled = true
	}

	if n.GetOnline() {
		d.Authorized = true
	}

	return d
}

// NodeToDeviceRoutes extracts route information from a headscale Node.
func NodeToDeviceRoutes(n *v1.Node) model.DeviceRoutes {
	return model.DeviceRoutes{
		AdvertisedRoutes: n.GetAvailableRoutes(),
		EnabledRoutes:    n.GetApprovedRoutes(),
	}
}
