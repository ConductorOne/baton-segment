package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-segment/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

type warehouseBuilder struct {
	client *client.Client
}

func (b *warehouseBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return warehouseResourceType
}

// List returns all warehouses in the workspace.
func (b *warehouseBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, opts rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)

	if parentResourceID == nil {
		return nil, nil, nil
	}

	bag, err := parsePageToken(opts.PageToken.Token, &v2.ResourceId{ResourceType: warehouseResourceType.Id})
	if err != nil {
		return nil, nil, err
	}

	pageToken := bag.PageToken()

	outputAnnotations := annotations.New()
	response, rateLimit, err := b.client.ListWarehouses(ctx, pageToken, client.DefaultPageSize)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to list warehouses: %w", err)
	}

	var resources []*v2.Resource
	for _, warehouse := range response.Data.Warehouses {
		displayName := warehouse.ID
		if warehouse.Metadata != nil && warehouse.Metadata.Name != "" {
			displayName = warehouse.Metadata.Name
		}

		profile := map[string]interface{}{
			"id":                  warehouse.ID,
			profileWorkspaceIDKey: warehouse.WorkspaceID,
			"enabled":             warehouse.Enabled,
		}
		if warehouse.Metadata != nil {
			profile["slug"] = warehouse.Metadata.Slug
			profile[profileNameKey] = warehouse.Metadata.Name
			profile["description"] = warehouse.Metadata.Description
		}

		resource, err := rs.NewResource(
			displayName,
			warehouseResourceType,
			warehouse.ID,
			rs.WithParentResourceID(parentResourceID),
			rs.WithAppTrait(
				rs.WithAppProfile(profile),
			),
		)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to create warehouse resource: %w", err)
		}
		resources = append(resources, resource)
	}

	nextToken, err := bag.NextToken(response.Data.Pagination.Next)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, err
	}

	l.Debug("listed warehouses", zap.Int("count", len(resources)))

	return resources, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: outputAnnotations}, nil
}

// Entitlements returns role-based entitlements for the warehouse.
func (b *warehouseBuilder) Entitlements(ctx context.Context, resource *v2.Resource, opts rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	outputAnnotations := annotations.New()

	entitlements, rateLimit, err := buildFilteredRoleEntitlements(ctx, b.client, opts.Session, resource, "Warehouse")
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to build role entitlements: %w", err)
	}

	return entitlements, &rs.SyncOpResults{Annotations: outputAnnotations}, nil
}

// Grants returns no grants for warehouses.
// Grants are synced from users and groups via their permissions.
func (b *warehouseBuilder) Grants(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

// Grant assigns a role to a user/group on the warehouse.
func (b *warehouseBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) ([]*v2.Grant, annotations.Annotations, error) {
	return grantRoleEntitlement(ctx, b.client, principal, entitlement)
}

// Revoke removes a role assignment from a user/group on the warehouse.
func (b *warehouseBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	return revokeRoleEntitlement(ctx, b.client, grant)
}

func newWarehouseBuilder(c *client.Client) *warehouseBuilder {
	return &warehouseBuilder{client: c}
}
