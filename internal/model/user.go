package model

// User represents a Tailscale user.
type User struct {
	ID                 string `json:"id"`
	DisplayName        string `json:"displayName"`
	LoginName          string `json:"loginName"`
	ProfilePicURL      string `json:"profilePicUrl,omitempty"`
	TailnetID          string `json:"tailnetId,omitempty"`
	Created            string `json:"created"`
	Role               string `json:"role,omitempty"`
	Status             string `json:"status,omitempty"`
	DeviceCount        int    `json:"deviceCount,omitempty"`
	CurrentlyConnected bool   `json:"currentlyConnected,omitempty"`
}

// UserList is the response for listing users.
type UserList struct {
	Users []User `json:"users"`
}
