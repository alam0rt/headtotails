package translate

import (
	"fmt"
	"time"

	v1 "github.com/juanfont/headscale/gen/go/headscale/v1"
	"github.com/alam0rt/headtotails/internal/model"
)

// UserToTailscaleUser converts a headscale proto User to a Tailscale API User.
func UserToTailscaleUser(u *v1.User) model.User {
	user := model.User{
		ID:            fmt.Sprintf("%d", u.GetId()),
		DisplayName:   u.GetDisplayName(),
		LoginName:     u.GetName(),
		ProfilePicURL: u.GetProfilePicUrl(),
		Status:        "active",
		Role:          "member",
	}

	if user.DisplayName == "" {
		user.DisplayName = u.GetName()
	}

	if ca := u.GetCreatedAt(); ca != nil {
		user.Created = ca.AsTime().Format(time.RFC3339)
	}

	return user
}

// FindUserByID finds a user in the list by string ID.
func FindUserByID(users []*v1.User, id string) *v1.User {
	for _, u := range users {
		if fmt.Sprintf("%d", u.GetId()) == id {
			return u
		}
	}
	return nil
}
