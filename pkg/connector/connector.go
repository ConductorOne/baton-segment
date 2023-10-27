package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/conductorone/baton-segment/pkg/segment"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

type Segment struct {
	client *segment.Client
}

// ResourceSyncers returns a ResourceSyncer for each resource type that should be synced from the upstream service.
func (s *Segment) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncer {
	return []connectorbuilder.ResourceSyncer{
		newUserBuilder(s.client),
		newWorkspaceBuilder(s.client),
		newGroupBuilder(s.client),
		newRoleBuilder(s.client),
		newRoleResourceBuilder(s.client),
	}
}

// Metadata returns metadata about the connector.
func (s *Segment) Metadata(ctx context.Context) (*v2.ConnectorMetadata, error) {
	return &v2.ConnectorMetadata{
		DisplayName: "Segment",
		Description: "Connector syncing Segment users, groups, roles, and workspaces.",
	}, nil
}

// Validate is called to ensure that the connector is properly configured. It should exercise any API credentials
// to be sure that they are valid.
func (s *Segment) Validate(ctx context.Context) (annotations.Annotations, error) {
	_, err := s.client.GetWorkspace(ctx)
	if err != nil {
		return nil, fmt.Errorf("error validating Segment connector: %w", err)
	}
	return nil, nil
}

// New returns a new instance of the connector.
func New(ctx context.Context, token string) (*Segment, error) {
	httpClient, err := uhttp.NewClient(ctx, uhttp.WithLogger(true, ctxzap.Extract(ctx)))
	if err != nil {
		return nil, err
	}

	client := segment.NewClient(httpClient, token)

	return &Segment{
		client: client,
	}, nil
}
