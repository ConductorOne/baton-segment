package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	gr "github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-segment/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

type userBuilder struct {
	client *client.Client
}

func (b *userBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return userResourceType
}

// List returns all the users from the Segment workspace.
func (b *userBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, opts rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)

	bag, err := parsePageToken(opts.PageToken.Token, &v2.ResourceId{ResourceType: userResourceType.Id})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse page token: %w", err)
	}

	pageToken := bag.PageToken()

	outputAnnotations := annotations.New()
	response, rateLimit, err := b.client.ListUsers(ctx, pageToken, client.DefaultPageSize)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to list users: %w", err)
	}

	var resources []*v2.Resource
	for _, user := range response.Data.Users {
		r, err := userResource(&user, parentResourceID)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to create user resource: %w", err)
		}
		resources = append(resources, r)
	}

	nextToken, err := bag.NextToken(response.Data.Pagination.Next)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to create next page token: %w", err)
	}

	l.Debug("listed users", zap.Int("count", len(resources)), zap.String("next_token", nextToken))

	return resources, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: outputAnnotations}, nil
}

// userResource creates a v2.Resource from a Segment user.
func userResource(user *client.User, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"id":           user.ID,
		profileNameKey: user.Name,
	}

	userTraitOptions := []rs.UserTraitOption{
		rs.WithEmail(user.Email, true),
	}

	if user.Email != "" {
		userTraitOptions = append(userTraitOptions, rs.WithUserLogin(user.Email))
	}

	displayName := user.Name
	if displayName == "" {
		displayName = user.Email
	}

	return rs.NewUserResource(
		displayName,
		userResourceType,
		user.ID,
		userTraitOptions,
		rs.WithResourceProfile(profile),
		rs.WithResourceStatus(v2.Status_RESOURCE_STATUS_ENABLED, ""), // Segment users are always enabled (deleted if inactive)
		rs.WithParentResourceID(parentResourceID),
	)
}

// Entitlements returns no entitlements for users (users receive grants, not entitlements).
func (b *userBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

// Grants returns the role grants for a user by fetching user details.
// Each grant represents a unique (scope resource, role) combination.
// Grants point to entitlements on scope resources (workspace, source, etc.).
func (b *userBuilder) Grants(ctx context.Context, resource *v2.Resource, opts rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)
	userID := resource.Id.Resource

	outputAnnotations := annotations.New()

	// Fetch user details to get permissions
	userResp, rateLimit, err := b.client.GetUser(ctx, userID)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to get user details: %w", err)
	}

	var grants []*v2.Grant
	for _, perm := range userResp.Data.User.Permissions {
		// Create a grant for each resource scope in the permission
		for _, res := range perm.Resources {
			scopeResourceType := getScopeResourceType(res.Type)
			if scopeResourceType == nil {
				l.Debug("unknown scope resource type, skipping",
					zap.String("resource_type", res.Type),
					zap.String("resource_id", res.ID),
				)
				continue
			}

			var grant *v2.Grant
			if res.Type == ResourceTypeWorkspace {
				// Workspace-scoped permissions: grant on role:{role_id}:member
				// This matches baton-segment grant IDs.
				roleRes := &v2.Resource{
					Id: &v2.ResourceId{
						ResourceType: roleResourceType.Id,
						Resource:     perm.RoleID,
					},
				}
				grant = gr.NewGrant(roleRes, roleMembership, resource.Id)
			} else {
				// Source/function/etc-scoped permissions: grant on scope resource with snake_case slug
				// This matches baton-segment entitlement IDs (e.g., source_admin, function_read_only).
				roleSlug, err := snakeifyRoleName(perm.RoleName)
				if err != nil {
					l.Debug("skipping permission with invalid role name",
						zap.String("role_id", perm.RoleID),
						zap.String("role_name", perm.RoleName),
						zap.Error(err),
					)
					continue
				}
				scopeResource := &v2.Resource{
					Id: &v2.ResourceId{
						ResourceType: scopeResourceType.Id,
						Resource:     res.ID,
					},
				}
				grant = gr.NewGrant(scopeResource, roleSlug, resource.Id)
			}
			grants = append(grants, grant)
		}
	}

	l.Debug("listed user grants", zap.String("user_id", userID), zap.Int("count", len(grants)))

	return grants, &rs.SyncOpResults{Annotations: outputAnnotations}, nil
}

// Delete removes a user from the Segment workspace.
func (b *userBuilder) Delete(ctx context.Context, resourceID *v2.ResourceId) (annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	if resourceID.ResourceType != userResourceType.Id {
		return nil, fmt.Errorf("invalid resource type: expected %s, got %s", userResourceType.Id, resourceID.ResourceType)
	}

	userID := resourceID.Resource

	l.Debug("deleting user", zap.String("user_id", userID))

	outputAnnotations := annotations.New()
	rateLimit, err := b.client.DeleteUser(ctx, userID)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return outputAnnotations, fmt.Errorf("failed to delete user: %w", err)
	}

	l.Debug("user deleted successfully", zap.String("user_id", userID))

	return outputAnnotations, nil
}

func newUserBuilder(c *client.Client) *userBuilder {
	return &userBuilder{client: c}
}
