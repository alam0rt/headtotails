package model

// Device represents a Tailscale device (maps to a headscale Node).
type Device struct {
	ID                        string   `json:"id"`
	Addresses                 []string `json:"addresses"`
	Name                      string   `json:"name"`
	Hostname                  string   `json:"hostname"`
	ClientVersion             string   `json:"clientVersion"`
	UpdateAvailable           bool     `json:"updateAvailable"`
	FullyQualifiedDomainName  string   `json:"fqdn,omitempty"` // FQDN TODO should be named "name"?
	OS                        string   `json:"os"`
	Created                   string   `json:"created"`
	LastSeen                  string   `json:"lastSeen"`
	KeyExpiryDisabled         bool     `json:"keyExpiryDisabled"`
	Expires                   string   `json:"expires"`
	Authorized                bool     `json:"authorized"`
	IsExternal                bool     `json:"isExternal"`
	MachineKey                string   `json:"machineKey"`
	NodeKey                   string   `json:"nodeKey"`
	User                      string   `json:"user"`
	Tags                      []string `json:"tags"`
	BlocksIncomingConnections bool     `json:"blocksIncomingConnections"`
}

// DeviceList is the response for listing devices.
type DeviceList struct {
	Devices []Device `json:"devices"`
}

// DeviceRoutes represents the routes for a device.
type DeviceRoutes struct {
	AdvertisedRoutes []string `json:"advertisedRoutes"`
	EnabledRoutes    []string `json:"enabledRoutes"`
}

// DeviceRoutesRequest is the request body for setting routes.
type DeviceRoutesRequest struct {
	Routes []string `json:"routes"`
}

// DeviceTagsRequest is the request body for setting device tags.
type DeviceTagsRequest struct {
	Tags []string `json:"tags"`
}

// DeviceNameRequest is the request body for renaming a device.
type DeviceNameRequest struct {
	Name string `json:"name"`
}

// DeviceAuthorizeRequest is the request body for authorizing a device.
type DeviceAuthorizeRequest struct {
	Authorized bool `json:"authorized"`
}
