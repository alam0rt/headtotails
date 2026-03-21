package model

// Error is the standard Tailscale API error response shape.
type Error struct {
	Message string `json:"message"`
}
