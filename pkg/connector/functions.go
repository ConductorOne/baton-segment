package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-segment/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

// functionResourceTypes are the valid resource types for the Segment Functions API.
// The API requires a resourceType parameter and doesn't support listing all at once.
var functionResourceTypes = []string{"SOURCE", "DESTINATION", "INSERT_DESTINATION"}

type functionBuilder struct {
	client *client.Client
}

func (b *functionBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return functionResourceType
}

// List returns all functions in the workspace.
// Functions are listed by resource type (SOURCE, DESTINATION, INSERT_DESTINATION).
func (b *functionBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, opts rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)

	if parentResourceID == nil {
		return nil, nil, nil
	}

	// Parse pagination token
	bag := &pagination.Bag{}
	if opts.PageToken.Token != "" {
		if err := bag.Unmarshal(opts.PageToken.Token); err != nil {
			return nil, nil, fmt.Errorf("failed to parse page token: %w", err)
		}
	}

	// Initialize pagination state on first call
	currentState := bag.Current()
	if currentState == nil {
		// Push resource types in reverse order (stack - last pushed = first processed)
		for i := len(functionResourceTypes) - 1; i >= 0; i-- {
			bag.Push(pagination.PageState{
				ResourceTypeID: functionResourceTypes[i],
				ResourceID:     parentResourceID.Resource,
			})
		}
		currentState = bag.Current()
	}

	// Get the current resource type we're listing
	resourceType := currentState.ResourceTypeID
	pageToken := bag.PageToken()

	outputAnnotations := annotations.New()
	response, rateLimit, err := b.client.ListFunctions(ctx, pageToken, client.DefaultPageSize, resourceType)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		l.Debug("failed to list functions",
			zap.Error(err),
			zap.String("resource_type", resourceType),
		)
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to list functions: %w", err)
	}

	var resources []*v2.Resource
	for _, fn := range response.Data.Functions {
		displayName := fn.DisplayName
		if displayName == "" {
			displayName = fn.ID
		}

		profile := map[string]interface{}{
			"id":            fn.ID,
			"workspace_id":  fn.WorkspaceID,
			"display_name":  fn.DisplayName,
			"description":   fn.Description,
			"resource_type": fn.ResourceType,
		}

		resource, err := rs.NewResource(
			displayName,
			functionResourceType,
			fn.ID,
			rs.WithParentResourceID(parentResourceID),
			rs.WithAppTrait(
				rs.WithAppProfile(profile),
			),
		)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to create function resource: %w", err)
		}
		resources = append(resources, resource)
	}

	// Handle pagination within current resource type, or move to next type
	nextToken, err := bag.NextToken(response.Data.Pagination.Next)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, err
	}

	l.Debug("listed functions",
		zap.String("resource_type", resourceType),
		zap.Int("count", len(resources)),
	)

	return resources, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: outputAnnotations}, nil
}

// Entitlements returns role-based entitlements for the function.
func (b *functionBuilder) Entitlements(ctx context.Context, resource *v2.Resource, opts rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)
	outputAnnotations := annotations.New()

	// Only roles containing "Function" in their name, using snake_case slugs
	// (e.g., function_admin, function_read_only).
	entitlements, rateLimit, err := buildFilteredRoleEntitlements(ctx, b.client, opts.Session, resource, "Function")
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		l.Debug("failed to build role entitlements for function", zap.Error(err))
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to build role entitlements: %w", err)
	}

	return entitlements, &rs.SyncOpResults{Annotations: outputAnnotations}, nil
}

// Grants returns no grants for functions.
// Grants are synced from users and groups via their permissions.
func (b *functionBuilder) Grants(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

// Grant assigns a role to a user/group on the function.
func (b *functionBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) ([]*v2.Grant, annotations.Annotations, error) {
	return grantRoleEntitlement(ctx, b.client, principal, entitlement)
}

// Revoke removes a role assignment from a user/group on the function.
func (b *functionBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	return revokeRoleEntitlement(ctx, b.client, grant)
}

func newFunctionBuilder(c *client.Client) *functionBuilder {
	return &functionBuilder{client: c}
}
