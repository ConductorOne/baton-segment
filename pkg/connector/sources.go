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

type sourceBuilder struct {
	client *client.Client
}

func (b *sourceBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return sourceResourceType
}

// List returns all sources in the workspace.
func (b *sourceBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, opts rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)

	if parentResourceID == nil {
		return nil, nil, nil
	}

	bag, err := parsePageToken(opts.PageToken.Token, &v2.ResourceId{ResourceType: sourceResourceType.Id})
	if err != nil {
		return nil, nil, err
	}

	pageToken := bag.PageToken()

	outputAnnotations := annotations.New()
	response, rateLimit, err := b.client.ListSources(ctx, pageToken, client.DefaultPageSize)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to list sources: %w", err)
	}

	var resources []*v2.Resource
	for _, source := range response.Data.Sources {
		displayName := source.Name
		if displayName == "" {
			displayName = source.Slug
		}

		profile := map[string]interface{}{
			"id":                  source.ID,
			"slug":                source.Slug,
			profileNameKey:        source.Name,
			profileWorkspaceIDKey: source.WorkspaceID,
			"enabled":             source.Enabled,
		}

		resource, err := rs.NewResource(
			displayName,
			sourceResourceType,
			source.ID,
			rs.WithParentResourceID(parentResourceID),
			rs.WithAppTrait(
				rs.WithAppProfile(profile),
			),
		)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to create source resource: %w", err)
		}
		resources = append(resources, resource)
	}

	nextToken, err := bag.NextToken(response.Data.Pagination.Next)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, err
	}

	l.Debug("listed sources", zap.Int("count", len(resources)))

	return resources, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: outputAnnotations}, nil
}

// Entitlements returns role-based entitlements for the source.
// Only roles containing "Source" in their name are included, using snake_case slugs
// (e.g., source_admin).
func (b *sourceBuilder) Entitlements(ctx context.Context, resource *v2.Resource, opts rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	outputAnnotations := annotations.New()

	entitlements, rateLimit, err := buildFilteredRoleEntitlements(ctx, b.client, opts.Session, resource, "Source")
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to build role entitlements: %w", err)
	}

	return entitlements, &rs.SyncOpResults{Annotations: outputAnnotations}, nil
}

// Grants returns no grants for sources.
// Grants are synced from users and groups via their permissions.
func (b *sourceBuilder) Grants(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

// Grant assigns a role to a user/group on the source.
func (b *sourceBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) ([]*v2.Grant, annotations.Annotations, error) {
	return grantRoleEntitlement(ctx, b.client, principal, entitlement)
}

// Revoke removes a role assignment from a user/group on the source.
func (b *sourceBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	return revokeRoleEntitlement(ctx, b.client, grant)
}

func newSourceBuilder(c *client.Client) *sourceBuilder {
	return &sourceBuilder{client: c}
}
