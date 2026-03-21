package model

// KeyCapabilityDevicesCreate describes what devices created with this key can do.
type KeyCapabilityDevicesCreate struct {
	Reusable      bool     `json:"reusable"`
	Ephemeral     bool     `json:"ephemeral"`
	Preauthorized bool     `json:"preauthorized"`
	Tags          []string `json:"tags,omitempty"`
}

// KeyCapabilityDevices holds the device creation capabilities.
type KeyCapabilityDevices struct {
	Create KeyCapabilityDevicesCreate `json:"create"`
}

// KeyCapability holds all capabilities for a key.
type KeyCapability struct {
	Devices KeyCapabilityDevices `json:"devices"`
}

// Key represents a Tailscale auth key.
type Key struct {
	ID           string        `json:"id"`
	Key          string        `json:"key,omitempty"`
	Description  string        `json:"description,omitempty"`
	Created      string        `json:"created"`
	Expires      string        `json:"expires"`
	Revoked      string        `json:"revoked,omitempty"`
	Invalid      bool          `json:"invalid"`
	Capabilities KeyCapability `json:"capabilities"`
}

// KeyList is the response for listing keys.
type KeyList struct {
	Keys []Key `json:"keys"`
}

// CreateKeyRequest is the request body for creating an auth key.
type CreateKeyRequest struct {
	Capabilities  KeyCapability `json:"capabilities"`
	ExpirySeconds int           `json:"expirySeconds,omitempty"`
	Description   string        `json:"description,omitempty"`
}
