package translate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPolicyToResponse(t *testing.T) {
	got := PolicyToResponse(`{"acls":[]}`)
	assert.Equal(t, map[string]string{"policy": `{"acls":[]}`}, got)
}
