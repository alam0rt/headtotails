package translate

import (
	"testing"
	"time"

	v1 "github.com/juanfont/headscale/gen/go/headscale/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestUserToTailscaleUser(t *testing.T) {
	created := timestamppb.New(time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC))
	in := &v1.User{
		Id:            10,
		Name:          "alice",
		DisplayName:   "Alice",
		ProfilePicUrl: "https://example.com/alice.png",
		CreatedAt:     created,
	}

	got := UserToTailscaleUser(in)
	assert.Equal(t, "10", got.ID)
	assert.Equal(t, "Alice", got.DisplayName)
	assert.Equal(t, "alice", got.LoginName)
	assert.Equal(t, "https://example.com/alice.png", got.ProfilePicURL)
	assert.Equal(t, created.AsTime().Format(time.RFC3339), got.Created)
	assert.Equal(t, "active", got.Status)
	assert.Equal(t, "member", got.Role)
}

func TestUserToTailscaleUserDisplayNameFallback(t *testing.T) {
	got := UserToTailscaleUser(&v1.User{Id: 5, Name: "fallback"})
	assert.Equal(t, "fallback", got.DisplayName)
}

func TestFindUserByID(t *testing.T) {
	users := []*v1.User{{Id: 1, Name: "a"}, {Id: 2, Name: "b"}}
	assert.Equal(t, users[1], FindUserByID(users, "2"))
	assert.Nil(t, FindUserByID(users, "3"))
}
