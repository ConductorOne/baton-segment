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

const roleMembership = "member"

type roleBuilder struct {
	client *client.Client
}

func (b *roleBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return roleResourceType
}

// List returns all the roles from the Segment workspace.
// Roles are system-defined by Segment (~8 built-in), so all are returned in one call.
func (b *roleBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, opts rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)
	outputAnnotations := annotations.New()

	roles, rateLimit, err := getCachedRoles(ctx, b.client, opts.Session)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to list roles: %w", err)
	}

	var resources []*v2.Resource
	for _, role := range roles {
		r, err := roleResource(&role, parentResourceID)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to create role resource: %w", err)
		}
		resources = append(resources, r)
	}

	l.Debug("listed roles", zap.Int("count", len(resources)))

	return resources, &rs.SyncOpResults{Annotations: outputAnnotations}, nil
}

// roleResource creates a v2.Resource from a Segment role.
func roleResource(role *client.Role, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"id": role.ID,
	}

	roleTraitOptions := []rs.RoleTraitOption{}

	return rs.NewRoleResource(
		role.Name,
		roleResourceType,
		role.ID,
		roleTraitOptions,
		rs.WithResourceProfile(profile),
		rs.WithParentResourceID(parentResourceID),
		rs.WithDescription(role.Description),
	)
}

// Entitlements is unused — static entitlements handle role membership.
func (b *roleBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

// StaticEntitlements returns a "member" entitlement template applied to all role resources.
// The SDK expands this once per role resource, matching the baton-segment grant model: role:{id}:member.
func (b *roleBuilder) StaticEntitlements(_ context.Context, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	e := v2.Entitlement_builder{
		Slug:    roleMembership,
		Purpose: v2.Entitlement_PURPOSE_VALUE_ASSIGNMENT,
		// Invites are grantable too: the role is attached at invite time so a person
		// without a Segment account can still be provisioned (see grantPermissionsToInvite).
		GrantableTo: []*v2.ResourceType{userResourceType, inviteResourceType},
	}.Build()
	return []*v2.Entitlement{e}, nil, nil
}

// Grants returns no grants from roles.
// Grants are synced from users pointing to role:*:member entitlements.
func (b *roleBuilder) Grants(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

// Grant assigns a workspace-scoped role to a user or to a pending invitation.
// The role ID is the entitlement's resource ID; the workspace ID is its parent.
//
// Invite principals have no Segment account to attach permissions to, so the role travels
// with the invitation (POST /invites) instead of going through the user permissions
// endpoint. See grantPermissionsToInvite.
func (b *roleBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) ([]*v2.Grant, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)
	outputAnnotations := annotations.New()

	if principal.Id.ResourceType != userResourceType.Id && !isInvitePrincipal(principal) {
		return nil, nil, fmt.Errorf("only users and invites can be granted role membership, got %s", principal.Id.ResourceType)
	}

	roleID := entitlement.Resource.Id.Resource
	workspaceID, rateLimit, err := resolveWorkspaceID(ctx, b.client, principal)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return nil, outputAnnotations, err
	}

	l.Debug("granting role membership",
		zap.String("role_id", roleID),
		zap.String("workspace_id", workspaceID),
		zap.String("principal_type", principal.Id.ResourceType),
		zap.String("principal_id", principal.Id.Resource),
	)

	permissions := []client.PermissionInput{
		{
			RoleID: roleID,
			Resources: []client.ResourceInput{
				{ID: workspaceID, Type: ResourceTypeWorkspace},
			},
		},
	}

	if isInvitePrincipal(principal) {
		email, err := principalEmail(principal)
		if err != nil {
			return nil, outputAnnotations, fmt.Errorf("failed to get email for invite principal: %w", err)
		}

		if err := grantPermissionsToInvite(ctx, b.client, email, permissions, &outputAnnotations); err != nil {
			return nil, outputAnnotations, err
		}
	} else {
		var addRL *v2.RateLimitDescription
		_, addRL, err = b.client.AddUserPermissions(ctx, principal.Id.Resource, permissions)
		outputAnnotations.WithRateLimiting(addRL)
		if err != nil {
			return nil, outputAnnotations, fmt.Errorf("failed to grant role membership: %w", err)
		}
	}

	grant := gr.NewGrant(entitlement.Resource, roleMembership, principal.Id)
	return []*v2.Grant{grant}, outputAnnotations, nil
}

// Revoke removes a workspace-scoped role from a user or from a pending invitation.
func (b *roleBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)
	outputAnnotations := annotations.New()

	principal := grant.Principal
	if principal.Id.ResourceType != userResourceType.Id && !isInvitePrincipal(principal) {
		return nil, fmt.Errorf("only users and invites can have role membership revoked, got %s", principal.Id.ResourceType)
	}

	roleID := grant.Entitlement.Resource.Id.Resource
	workspaceID, rl, err := resolveWorkspaceID(ctx, b.client, principal)
	outputAnnotations.WithRateLimiting(rl)
	if err != nil {
		return outputAnnotations, err
	}

	l.Debug("revoking role membership",
		zap.String("role_id", roleID),
		zap.String("workspace_id", workspaceID),
		zap.String("principal_type", principal.Id.ResourceType),
		zap.String("principal_id", principal.Id.Resource),
	)

	if isInvitePrincipal(principal) {
		email, err := principalEmail(principal)
		if err != nil {
			return outputAnnotations, fmt.Errorf("failed to get email for invite principal: %w", err)
		}

		if err := revokeRoleFromInvite(ctx, b.client, email, roleID, ResourceTypeWorkspace, workspaceID, &outputAnnotations); err != nil {
			return outputAnnotations, err
		}

		return outputAnnotations, nil
	}

	if err := revokeUserRolePermission(ctx, b.client, principal.Id.Resource, roleID, ResourceTypeWorkspace, workspaceID, &outputAnnotations); err != nil {
		return outputAnnotations, err
	}

	return outputAnnotations, nil
}

func newRoleBuilder(c *client.Client) *roleBuilder {
	return &roleBuilder{client: c}
}
