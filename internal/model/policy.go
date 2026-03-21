package model

// Policy represents a Tailscale policy (ACL) document.
type Policy struct {
	Policy  string `json:"policy,omitempty"`
	ETag    string `json:"etag,omitempty"`
	Revised string `json:"revised,omitempty"`
}
