package headscale

import (
"context"
"fmt"

v1 "github.com/juanfont/headscale/gen/go/headscale/v1"
"google.golang.org/grpc"
"google.golang.org/grpc/credentials/insecure"
"google.golang.org/grpc/metadata"
)

// HeadscaleClient is the central seam for testing — all handlers receive this interface.
type HeadscaleClient interface {
	// Nodes
	ListNodes(ctx context.Context, user string) ([]*v1.Node, error)
	GetNode(ctx context.Context, id uint64) (*v1.Node, error)
	DeleteNode(ctx context.Context, id uint64) error
	ExpireNode(ctx context.Context, id uint64) error
	RenameNode(ctx context.Context, id uint64, name string) (*v1.Node, error)
	SetTags(ctx context.Context, id uint64, tags []string) (*v1.Node, error)
	SetApprovedRoutes(ctx context.Context, id uint64, routes []string) (*v1.Node, error)
	AuthApprove(ctx context.Context, user, nodeKey string) (*v1.Node, error)

	// PreAuthKeys
	ListPreAuthKeys(ctx context.Context) ([]*v1.PreAuthKey, error)
	CreatePreAuthKey(ctx context.Context, req *v1.CreatePreAuthKeyRequest) (*v1.PreAuthKey, error)
	ExpirePreAuthKey(ctx context.Context, id uint64) error
	DeletePreAuthKey(ctx context.Context, id uint64) error

	// Users
	ListUsers(ctx context.Context) ([]*v1.User, error)
	DeleteUser(ctx context.Context, id uint64) error

	// Policy
	GetPolicy(ctx context.Context) (string, error)
	SetPolicy(ctx context.Context, policy string) error
}

// GRPCClient is the real implementation of HeadscaleClient backed by gRPC.
type GRPCClient struct {
	client v1.HeadscaleServiceClient
	apiKey string
}

// New creates a new GRPCClient connected to addr with the given API key.
func New(addr, apiKey string) (*GRPCClient, error) {
	conn, err := grpc.NewClient(addr,
grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial headscale gRPC at %s: %w", addr, err)
	}
	return &GRPCClient{
		client: v1.NewHeadscaleServiceClient(conn),
		apiKey: apiKey,
	}, nil
}

func (c *GRPCClient) withAPIKey(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.apiKey)
}

func (c *GRPCClient) ListNodes(ctx context.Context, user string) ([]*v1.Node, error) {
	resp, err := c.client.ListNodes(c.withAPIKey(ctx), &v1.ListNodesRequest{User: user})
	if err != nil {
		return nil, err
	}
	return resp.Nodes, nil
}

func (c *GRPCClient) GetNode(ctx context.Context, id uint64) (*v1.Node, error) {
	resp, err := c.client.GetNode(c.withAPIKey(ctx), &v1.GetNodeRequest{NodeId: id})
	if err != nil {
		return nil, err
	}
	return resp.Node, nil
}

func (c *GRPCClient) DeleteNode(ctx context.Context, id uint64) error {
	_, err := c.client.DeleteNode(c.withAPIKey(ctx), &v1.DeleteNodeRequest{NodeId: id})
	return err
}

func (c *GRPCClient) ExpireNode(ctx context.Context, id uint64) error {
	_, err := c.client.ExpireNode(c.withAPIKey(ctx), &v1.ExpireNodeRequest{NodeId: id})
	return err
}

func (c *GRPCClient) RenameNode(ctx context.Context, id uint64, name string) (*v1.Node, error) {
	resp, err := c.client.RenameNode(c.withAPIKey(ctx), &v1.RenameNodeRequest{
		NodeId:  id,
		NewName: name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Node, nil
}

func (c *GRPCClient) SetTags(ctx context.Context, id uint64, tags []string) (*v1.Node, error) {
	resp, err := c.client.SetTags(c.withAPIKey(ctx), &v1.SetTagsRequest{
		NodeId: id,
		Tags:   tags,
	})
	if err != nil {
		return nil, err
	}
	return resp.Node, nil
}

func (c *GRPCClient) SetApprovedRoutes(ctx context.Context, id uint64, routes []string) (*v1.Node, error) {
	resp, err := c.client.SetApprovedRoutes(c.withAPIKey(ctx), &v1.SetApprovedRoutesRequest{
		NodeId: id,
		Routes: routes,
	})
	if err != nil {
		return nil, err
	}
	return resp.Node, nil
}

func (c *GRPCClient) AuthApprove(ctx context.Context, user, nodeKey string) (*v1.Node, error) {
	resp, err := c.client.RegisterNode(c.withAPIKey(ctx), &v1.RegisterNodeRequest{
		User: user,
		Key:  nodeKey,
	})
	if err != nil {
		return nil, err
	}
	return resp.Node, nil
}

func (c *GRPCClient) ListPreAuthKeys(ctx context.Context) ([]*v1.PreAuthKey, error) {
	resp, err := c.client.ListPreAuthKeys(c.withAPIKey(ctx), &v1.ListPreAuthKeysRequest{})
	if err != nil {
		return nil, err
	}
	return resp.PreAuthKeys, nil
}

func (c *GRPCClient) CreatePreAuthKey(ctx context.Context, req *v1.CreatePreAuthKeyRequest) (*v1.PreAuthKey, error) {
	resp, err := c.client.CreatePreAuthKey(c.withAPIKey(ctx), req)
	if err != nil {
		return nil, err
	}
	return resp.PreAuthKey, nil
}

func (c *GRPCClient) ExpirePreAuthKey(ctx context.Context, id uint64) error {
	_, err := c.client.ExpirePreAuthKey(c.withAPIKey(ctx), &v1.ExpirePreAuthKeyRequest{Id: id})
	return err
}

func (c *GRPCClient) DeletePreAuthKey(ctx context.Context, id uint64) error {
	_, err := c.client.DeletePreAuthKey(c.withAPIKey(ctx), &v1.DeletePreAuthKeyRequest{Id: id})
	return err
}

func (c *GRPCClient) ListUsers(ctx context.Context) ([]*v1.User, error) {
	resp, err := c.client.ListUsers(c.withAPIKey(ctx), &v1.ListUsersRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Users, nil
}

func (c *GRPCClient) DeleteUser(ctx context.Context, id uint64) error {
	_, err := c.client.DeleteUser(c.withAPIKey(ctx), &v1.DeleteUserRequest{Id: id})
	return err
}

func (c *GRPCClient) GetPolicy(ctx context.Context) (string, error) {
	resp, err := c.client.GetPolicy(c.withAPIKey(ctx), &v1.GetPolicyRequest{})
	if err != nil {
		return "", err
	}
	return resp.Policy, nil
}

func (c *GRPCClient) SetPolicy(ctx context.Context, policy string) error {
	_, err := c.client.SetPolicy(c.withAPIKey(ctx), &v1.SetPolicyRequest{Policy: policy})
	return err
}
