package headscale

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
)

func TestWithAPIKeyAddsAuthorizationMetadata(t *testing.T) {
	c := &GRPCClient{apiKey: "hskey-api-abc"}
	ctx := c.withAPIKey(context.Background())

	md, ok := metadata.FromOutgoingContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, []string{"Bearer hskey-api-abc"}, md.Get("authorization"))
}
