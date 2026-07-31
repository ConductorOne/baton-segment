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

type spaceBuilder struct {
	client *client.Client
}

func (b *spaceBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return spaceResourceType
}

// List returns all spaces in the workspace.
func (b *spaceBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, opts rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)

	if parentResourceID == nil {
		return nil, nil, nil
	}

	bag, err := parsePageToken(opts.PageToken.Token, &v2.ResourceId{ResourceType: spaceResourceType.Id})
	if err != nil {
		return nil, nil, err
	}

	pageToken := bag.PageToken()

	outputAnnotations := annotations.New()
	response, rateLimit, err := b.client.ListSpaces(ctx, pageToken, client.DefaultPageSize)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to list spaces: %w", err)
	}

	var resources []*v2.Resource
	for _, space := range response.Data.Spaces {
		displayName := space.Name
		if displayName == "" {
			displayName = space.Slug
		}

		profile := map[string]interface{}{
			"id":           space.ID,
			profileNameKey: space.Name,
			"slug":         space.Slug,
		}

		resource, err := rs.NewResource(
			displayName,
			spaceResourceType,
			space.ID,
			rs.WithParentResourceID(parentResourceID),
			rs.WithAppTrait(),
			rs.WithResourceProfile(profile),
		)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to create space resource: %w", err)
		}
		resources = append(resources, resource)
	}

	nextToken, err := bag.NextToken(response.Data.Pagination.Next)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, err
	}

	l.Debug("listed spaces", zap.Int("count", len(resources)))

	return resources, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: outputAnnotations}, nil
}

// Entitlements returns role-based entitlements for the space.
func (b *spaceBuilder) Entitlements(ctx context.Context, resource *v2.Resource, opts rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)
	outputAnnotations := annotations.New()

	// Segment's space-scoped roles use "Engage" in their names, not "Space".
	entitlements, rateLimit, err := buildFilteredRoleEntitlements(ctx, b.client, opts.Session, resource, "Engage")
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		l.Debug("failed to build role entitlements for space", zap.Error(err))
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to build role entitlements: %w", err)
	}

	return entitlements, &rs.SyncOpResults{Annotations: outputAnnotations}, nil
}

// Grants returns no grants for spaces.
// Grants are synced from users and groups via their permissions.
func (b *spaceBuilder) Grants(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

// Grant assigns a role to a user/group on the space.
func (b *spaceBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) ([]*v2.Grant, annotations.Annotations, error) {
	return grantRoleEntitlement(ctx, b.client, principal, entitlement)
}

// Revoke removes a role assignment from a user/group on the space.
func (b *spaceBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	return revokeRoleEntitlement(ctx, b.client, grant)
}

func newSpaceBuilder(c *client.Client) *spaceBuilder {
	return &spaceBuilder{client: c}
}
