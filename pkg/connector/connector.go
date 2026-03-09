package connector

import (
	"context"
	"fmt"
	"io"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/cli"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	cfg "github.com/conductorone/baton-segment/pkg/config"
	"github.com/conductorone/baton-segment/pkg/connector/client"
)

type Connector struct {
	client *client.Client
}

// ResourceSyncers returns a ResourceSyncer for each resource type that should be synced.
// Workspace is first as it's the parent resource for all others.
func (c *Connector) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncerV2 {
	return []connectorbuilder.ResourceSyncerV2{
		newWorkspaceBuilder(c.client),
		newUserBuilder(c.client),
		newGroupBuilder(c.client),
		newRoleBuilder(c.client),
		newInviteBuilder(c.client),
		newSourceBuilder(c.client),
		newWarehouseBuilder(c.client),
		newFunctionBuilder(c.client),
		newSpaceBuilder(c.client),
	}
}

// Asset takes an input AssetRef and attempts to fetch it using the connector's authenticated http client.
func (c *Connector) Asset(ctx context.Context, asset *v2.AssetRef) (string, io.ReadCloser, error) {
	return "", nil, nil
}

// Metadata returns metadata about the connector.
func (c *Connector) Metadata(ctx context.Context) (*v2.ConnectorMetadata, error) {
	return &v2.ConnectorMetadata{
		DisplayName:           "Twilio Segment",
		Description:           "Connector for Twilio Segment",
		AccountCreationSchema: accountCreationSchema(),
	}, nil
}

// accountCreationSchema returns the schema for creating new user accounts (invites).
func accountCreationSchema() *v2.ConnectorAccountCreationSchema {
	return &v2.ConnectorAccountCreationSchema{
		FieldMap: map[string]*v2.ConnectorAccountCreationSchema_Field{
			"email": {
				DisplayName: "Email Address",
				Required:    true,
				Description: "Email address for the new user invitation",
				Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
					StringField: &v2.ConnectorAccountCreationSchema_StringField{},
				},
				Placeholder: "user@example.com",
				Order:       0,
			},
		},
	}
}

// Validate is called to ensure that the connector is properly configured.
func (c *Connector) Validate(ctx context.Context) (annotations.Annotations, error) {
	outputAnnotations := annotations.New()

	rateLimit, err := c.client.ValidateCredentials(ctx)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return outputAnnotations, fmt.Errorf("failed to validate credentials: %w", err)
	}

	return outputAnnotations, nil
}

// New returns a new instance of the connector.
func New(ctx context.Context,
	connectorConfig *cfg.Segment,
	cliOpts *cli.ConnectorOpts,
) (connectorbuilder.ConnectorBuilderV2,
	[]connectorbuilder.Opt,
	error,
) {
	c, err := client.New(ctx, connectorConfig.AccessToken, connectorConfig.BaseUrl)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &Connector{client: c}, nil, nil
}
