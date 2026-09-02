package connector

import (
	"context"
	"errors"
	"fmt"
	"strings"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/session"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	gr "github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/sessions"
	"github.com/conductorone/baton-segment/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	profileNameKey        = "name"
	profileWorkspaceIDKey = "workspace_id"
)

// errRoleNotFound is returned when a role slug cannot be matched to any existing role.
var errRoleNotFound = errors.New("role not found")

// isNotFoundError returns true when the Segment API responds with HTTP 404.
// uhttp maps HTTP 404 to gRPC codes.NotFound, so we use the gRPC status code
// rather than string-matching the error message.
func isNotFoundError(err error) bool {
	return status.Code(err) == codes.NotFound
}

// resolveWorkspaceID returns the workspace ID from the principal's ParentResourceId.
// If ParentResourceId is nil (e.g., SDK strips it in service mode), it falls back to
// fetching the workspace via the API.
func resolveWorkspaceID(ctx context.Context, c *client.Client, principal *v2.Resource) (string, *v2.RateLimitDescription, error) {
	if principal.ParentResourceId != nil {
		return principal.ParentResourceId.Resource, nil, nil
	}

	resp, rateLimit, err := c.GetWorkspace(ctx)
	if err != nil {
		return "", rateLimit, fmt.Errorf("principal missing parent resource ID and failed to fetch workspace: %w", err)
	}
	return resp.Data.Workspace.ID, rateLimit, nil
}

// Session cache key for the full roles list.
const rolesCacheKeyAll = "roles:all"

// getCachedRoles returns all roles, checking session cache first, then fetching from the API.
// Roles are system-defined by Segment (~8 built-in), so all are returned in one call.
func getCachedRoles(ctx context.Context, c *client.Client, ss sessions.SessionStore) ([]client.Role, *v2.RateLimitDescription, error) {
	if ss != nil {
		cached, found, err := session.GetJSON[[]client.Role](ctx, ss, rolesCacheKeyAll)
		if err == nil && found {
			return cached, nil, nil
		}
	}

	response, rateLimit, err := c.ListRoles(ctx)
	if err != nil {
		return nil, rateLimit, fmt.Errorf("failed to list roles: %w", err)
	}

	if ss != nil {
		_ = session.SetJSON(ctx, ss, rolesCacheKeyAll, response.Data.Roles)
	}

	return response.Data.Roles, rateLimit, nil
}

// parsePageToken parses a pagination token and returns a bag with the current state.
func parsePageToken(token string, resourceID *v2.ResourceId) (*pagination.Bag, error) {
	bag := &pagination.Bag{}
	if token != "" {
		if err := bag.Unmarshal(token); err != nil {
			return nil, err
		}
	}

	if bag.Current() == nil {
		bag.Push(pagination.PageState{
			ResourceTypeID: resourceID.ResourceType,
			ResourceID:     resourceID.Resource,
		})
	}

	return bag, nil
}

// snakeifyRoleName converts a role name to a snake_case slug.
// Example: "Source Admin" -> "source_admin", "Function Read-only" -> "function_read_only".
// Used for source/function entitlement slugs.
func snakeifyRoleName(name string) (string, error) {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "_")
	slug = strings.ReplaceAll(slug, "-", "_")

	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			_, _ = result.WriteRune(r)
		}
	}

	if result.Len() == 0 {
		return "", fmt.Errorf("role name produces an empty slug: %q", name)
	}

	return result.String(), nil
}

// findRoleBySlug looks up a role by its snake_case slug.
func findRoleBySlug(ctx context.Context, c *client.Client, slug string) (*client.Role, *v2.RateLimitDescription, error) {
	response, rateLimit, err := c.ListRoles(ctx)
	if err != nil {
		return nil, rateLimit, fmt.Errorf("failed to list roles: %w", err)
	}

	for _, role := range response.Data.Roles {
		roleSlug, err := snakeifyRoleName(role.Name)
		if err != nil {
			// Skip roles with invalid names.
			continue
		}
		if roleSlug == slug {
			return &role, rateLimit, nil
		}
	}

	return nil, rateLimit, fmt.Errorf("%w: %s", errRoleNotFound, slug)
}

// buildFilteredRoleEntitlements creates entitlements for roles whose name contains nameFilter,
// using snake_case slugs to match baton-segment entitlement IDs.
// Example: nameFilter="Source" produces source_admin; nameFilter="Function" produces function_admin.
func buildFilteredRoleEntitlements(ctx context.Context, c *client.Client, ss sessions.SessionStore, resource *v2.Resource, nameFilter string) ([]*v2.Entitlement, *v2.RateLimitDescription, error) {
	l := ctxzap.Extract(ctx)

	roles, rateLimit, err := getCachedRoles(ctx, c, ss)
	if err != nil {
		return nil, rateLimit, fmt.Errorf("failed to get cached roles for entitlements: %w", err)
	}

	var entitlements []*v2.Entitlement
	for _, role := range roles {
		if !strings.Contains(role.Name, nameFilter) {
			continue
		}

		roleSlug, err := snakeifyRoleName(role.Name)
		if err != nil {
			l.Debug("skipping role with invalid name", zap.String("role_id", role.ID), zap.Error(err))
			continue
		}

		e := ent.NewPermissionEntitlement(
			resource,
			roleSlug,
			ent.WithDisplayName(fmt.Sprintf("%s %s", resource.DisplayName, role.Name)),
			ent.WithDescription(fmt.Sprintf("%s - %s", role.Name, role.Description)),
			// Invites are grantable too: the role is attached at invite time so a person
			// without a Segment account can still be provisioned (see grantPermissionsToInvite).
			ent.WithGrantableTo(userResourceType, inviteResourceType),
		)
		entitlements = append(entitlements, e)
	}

	l.Debug("built filtered role entitlements",
		zap.String("resource_type", resource.Id.ResourceType),
		zap.String("resource_id", resource.Id.Resource),
		zap.String("name_filter", nameFilter),
		zap.Int("entitlement_count", len(entitlements)),
	)

	return entitlements, rateLimit, nil
}

// grantRoleEntitlement assigns a role to a principal on a scope resource.
// Extracts the role slug from the immutable entitlement ID, then looks up the role by slug.
func grantRoleEntitlement(ctx context.Context, c *client.Client, principal *v2.Resource, entitlement *v2.Entitlement) ([]*v2.Grant, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)
	outputAnnotations := annotations.New()

	if principal.Id.ResourceType != userResourceType.Id && !isInvitePrincipal(principal) {
		return nil, nil, fmt.Errorf("only users and invites can be granted role assignments, got %s", principal.Id.ResourceType)
	}

	// Extract slug from the immutable entitlement ID (not the editable Slug field).
	roleSlug, err := extractSlugFromEntitlementID(entitlement.Id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to extract role slug from entitlement ID: %w", err)
	}

	role, rateLimit, err := findRoleBySlug(ctx, c, roleSlug)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return nil, outputAnnotations, fmt.Errorf("failed to find role for entitlement %s: %w", roleSlug, err)
	}

	scopeResourceID := entitlement.Resource.Id.Resource
	scopeResourceType := strings.ToUpper(entitlement.Resource.Id.ResourceType)

	l.Debug("granting role on scope resource",
		zap.String("role_id", role.ID),
		zap.String("role_name", role.Name),
		zap.String("scope_type", scopeResourceType),
		zap.String("scope_id", scopeResourceID),
		zap.String("principal_type", principal.Id.ResourceType),
		zap.String("principal_id", principal.Id.Resource),
	)

	permissions := []client.PermissionInput{
		{
			RoleID: role.ID,
			Resources: []client.ResourceInput{
				{ID: scopeResourceID, Type: scopeResourceType},
			},
		},
	}

	if isInvitePrincipal(principal) {
		// The principal has no Segment account yet, so the role is attached to the
		// invitation itself instead of to a user.
		email, err := principalEmail(principal)
		if err != nil {
			return nil, outputAnnotations, fmt.Errorf("failed to get email for invite principal: %w", err)
		}

		if err := grantPermissionsToInvite(ctx, c, email, permissions, &outputAnnotations); err != nil {
			return nil, outputAnnotations, err
		}

		return []*v2.Grant{gr.NewGrant(entitlement.Resource, roleSlug, principal.Id)}, outputAnnotations, nil
	}

	_, rl, err := c.AddUserPermissions(ctx, principal.Id.Resource, permissions)
	outputAnnotations.WithRateLimiting(rl)
	if err != nil {
		return nil, outputAnnotations, fmt.Errorf("failed to add user permissions: %w", err)
	}

	grant := gr.NewGrant(entitlement.Resource, roleSlug, principal.Id)

	l.Debug("role granted successfully",
		zap.String("role_id", role.ID),
		zap.String("scope_type", scopeResourceType),
		zap.String("scope_id", scopeResourceID),
		zap.String("principal_id", principal.Id.Resource),
	)

	return []*v2.Grant{grant}, outputAnnotations, nil
}

// revokeRoleEntitlement removes a role assignment from a principal on a scope resource.
// Since Segment has no DELETE endpoint for individual permissions, this function:
// 1. Extracts the role slug from the immutable entitlement ID
// 2. Looks up the role by slug to get the role ID
// 3. Fetches the principal's current permissions
// 4. Filters out the target role/resource
// 5. Replaces all permissions with the filtered list (PUT).
//
// Invite principals hold their permissions on the invitation instead of on a user, so
// they take the revokeRoleFromInvite path.
func revokeRoleEntitlement(ctx context.Context, c *client.Client, grant *v2.Grant) (annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)
	outputAnnotations := annotations.New()

	principal := grant.Principal
	if principal.Id.ResourceType != userResourceType.Id && !isInvitePrincipal(principal) {
		return nil, fmt.Errorf("only users and invites can have role assignments revoked, got %s", principal.Id.ResourceType)
	}

	// Extract slug from the immutable entitlement ID (not the editable Slug field).
	roleSlug, err := extractSlugFromEntitlementID(grant.Entitlement.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to extract role slug from entitlement ID: %w", err)
	}

	role, rateLimit, err := findRoleBySlug(ctx, c, roleSlug)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		if errors.Is(err, errRoleNotFound) {
			l.Debug("role no longer exists, treating as already revoked",
				zap.String("role_slug", roleSlug),
			)
			outputAnnotations.Append(&v2.GrantAlreadyRevoked{})
			return outputAnnotations, nil
		}
		return outputAnnotations, fmt.Errorf("failed to find role for entitlement %s: %w", roleSlug, err)
	}

	scopeResourceID := grant.Entitlement.Resource.Id.Resource
	scopeResourceType := strings.ToUpper(grant.Entitlement.Resource.Id.ResourceType)

	l.Debug("revoking role on scope resource",
		zap.String("role_id", role.ID),
		zap.String("role_name", role.Name),
		zap.String("scope_type", scopeResourceType),
		zap.String("scope_id", scopeResourceID),
		zap.String("principal_type", principal.Id.ResourceType),
		zap.String("principal_id", principal.Id.Resource),
	)

	if isInvitePrincipal(principal) {
		email, err := principalEmail(principal)
		if err != nil {
			return outputAnnotations, fmt.Errorf("failed to get email for invite principal: %w", err)
		}

		if err := revokeRoleFromInvite(ctx, c, email, role.ID, scopeResourceType, scopeResourceID, &outputAnnotations); err != nil {
			return outputAnnotations, err
		}

		return outputAnnotations, nil
	}

	// Fetch the user's permissions, drop the target one and replace the list.
	if err := revokeUserRolePermission(ctx, c, principal.Id.Resource, role.ID, scopeResourceType, scopeResourceID, &outputAnnotations); err != nil {
		return outputAnnotations, err
	}

	l.Debug("role revoked successfully",
		zap.String("role_id", role.ID),
		zap.String("scope_type", scopeResourceType),
		zap.String("scope_id", scopeResourceID),
		zap.String("principal_id", principal.Id.Resource),
	)

	return outputAnnotations, nil
}

// extractSlugFromEntitlementID extracts the slug (last segment) from an entitlement ID.
// Entitlement ID format: {resourceType}:{resourceId}:{slug}.
func extractSlugFromEntitlementID(entitlementID string) (string, error) {
	parts := strings.Split(entitlementID, ":")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid entitlement ID format: %s", entitlementID)
	}
	return parts[len(parts)-1], nil
}

// filterOutPermission returns a new permissions list with the specified role/resource combination removed.
// If removing the resource leaves a role with no resources, the entire role entry is dropped.
func filterOutPermission(permissions []client.Permission, roleID, resourceType, resourceID string) []client.PermissionInput {
	var result []client.PermissionInput
	for _, perm := range permissions {
		if perm.RoleID != roleID {
			var resources []client.ResourceInput
			for _, r := range perm.Resources {
				resources = append(resources, client.ResourceInput{ID: r.ID, Type: r.Type})
			}
			result = append(result, client.PermissionInput{
				RoleID:    perm.RoleID,
				Resources: resources,
			})
			continue
		}

		var remaining []client.ResourceInput
		for _, r := range perm.Resources {
			if r.Type != resourceType || r.ID != resourceID {
				remaining = append(remaining, client.ResourceInput{ID: r.ID, Type: r.Type})
			}
		}
		if len(remaining) > 0 {
			result = append(result, client.PermissionInput{
				RoleID:    perm.RoleID,
				Resources: remaining,
			})
		}
	}

	return result
}

// getScopeResourceType converts a Segment resource type string to the corresponding v2.ResourceType.
func getScopeResourceType(segmentResourceType string) *v2.ResourceType {
	switch strings.ToUpper(segmentResourceType) {
	case ResourceTypeWorkspace:
		return workspaceResourceType
	case ResourceTypeSource:
		return sourceResourceType
	case ResourceTypeWarehouse:
		return warehouseResourceType
	case ResourceTypeFunction:
		return functionResourceType
	case ResourceTypeSpace:
		return spaceResourceType
	default:
		return nil
	}
}

// isInvitePrincipal reports whether the principal is a pending invitation rather than a
// provisioned Segment user.
func isInvitePrincipal(principal *v2.Resource) bool {
	return principal.GetId().GetResourceType() == inviteResourceType.Id
}

// principalEmail returns the email address for a user or invite principal.
// Invite resources carry the email as their resource ID (see inviteResource).
func principalEmail(principal *v2.Resource) (string, error) {
	email, traitErr := getEmailFromResource(principal)
	if traitErr == nil && email != "" {
		return email, nil
	}

	if isInvitePrincipal(principal) && principal.GetId().GetResource() != "" {
		return principal.GetId().GetResource(), nil
	}

	if traitErr != nil {
		return "", traitErr
	}
	return "", fmt.Errorf("principal %s has no email address", principal.GetId().GetResource())
}

// isAlreadyInvitedError returns true when Segment refuses an invite because the email
// already has a pending invitation. POST /invites answers 409 for a duplicate, which
// uhttp maps to codes.AlreadyExists; the message is also checked because Segment returns
// 422 ValidationFailure for some duplicate shapes.
func isAlreadyInvitedError(err error) bool {
	if err == nil {
		return false
	}
	if status.Code(err) == codes.AlreadyExists {
		return true
	}

	msg := strings.ToLower(err.Error())
	for _, needle := range []string{"already invited", "already been invited", "already exists", "duplicate"} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

// grantPermissionsToInvite assigns permissions to an invited email in a single
// POST /invites call.
//
// Segment's permission endpoints (POST/PUT /users/{id}/permissions) only accept
// provisioned users, so a role cannot be granted to somebody who has not accepted their
// invitation yet — that is the "only users can be granted role membership, got invite"
// failure this replaces. POST /invites takes the same permission payload at invite time,
// which is the only way to give a pending invite a role.
//
// Segment exposes no endpoint to read or modify a pending invite's permissions, so when
// the email already has an invitation (the usual case: ConductorOne creates the account
// and then grants the role) the invitation is withdrawn and re-issued carrying the
// permissions.
func grantPermissionsToInvite(
	ctx context.Context,
	c *client.Client,
	email string,
	permissions []client.PermissionInput,
	outputAnnotations *annotations.Annotations,
) error {
	l := ctxzap.Extract(ctx)

	inviteReq := []client.InviteRequest{{Email: email, Permissions: permissions}}

	_, rateLimit, err := c.CreateInvites(ctx, inviteReq)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err == nil {
		l.Debug("invited email with permissions", zap.String("email", email))
		return nil
	}
	if !isAlreadyInvitedError(err) {
		return fmt.Errorf("failed to invite %s with permissions: %w", email, err)
	}

	l.Info("email already has a pending invitation, re-issuing it with the requested permissions",
		zap.String("email", email),
	)

	rateLimit, deleteErr := c.DeleteInvites(ctx, []string{email})
	outputAnnotations.WithRateLimiting(rateLimit)
	if deleteErr != nil {
		return fmt.Errorf("failed to withdraw the existing invitation for %s: %w", email, deleteErr)
	}

	_, rateLimit, err = c.CreateInvites(ctx, inviteReq)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return fmt.Errorf("failed to re-issue the invitation for %s with permissions: %w", email, err)
	}

	l.Debug("re-issued invitation with permissions", zap.String("email", email))

	return nil
}

// revokeRoleFromInvite removes a role that was attached to an invite principal.
//
// Segment bakes permissions into the invitation and offers no endpoint to read or change
// them, so a still-pending invitation is withdrawn: that is the only way to stop the role
// from being applied when the invitation is accepted. Re-inviting is cheap, whereas
// leaving a pending invitation that still carries the revoked role would mean the revoke
// silently did nothing.
//
// If the invitation is no longer pending, the person either accepted it — so the
// permission is removed from the resulting user — or it was already withdrawn, which is
// reported as an already-revoked grant.
func revokeRoleFromInvite(
	ctx context.Context,
	c *client.Client,
	email string,
	roleID, scopeResourceType, scopeResourceID string,
	outputAnnotations *annotations.Annotations,
) error {
	l := ctxzap.Extract(ctx)

	pending, err := isInvitePending(ctx, c, email, outputAnnotations)
	if err != nil {
		return err
	}

	if pending {
		l.Warn("withdrawing pending invitation to revoke its role: Segment cannot modify the permissions of a pending invite",
			zap.String("email", email),
			zap.String("role_id", roleID),
		)

		rateLimit, err := c.DeleteInvites(ctx, []string{email})
		outputAnnotations.WithRateLimiting(rateLimit)
		if err != nil {
			return fmt.Errorf("failed to withdraw the invitation for %s: %w", email, err)
		}
		return nil
	}

	user, err := findUserByEmail(ctx, c, email, outputAnnotations)
	if err != nil {
		return err
	}
	if user == nil {
		l.Debug("no pending invitation and no user for email, treating as already revoked", zap.String("email", email))
		outputAnnotations.Append(&v2.GrantAlreadyRevoked{})
		return nil
	}

	l.Debug("invitation was accepted, revoking the role from the resulting user",
		zap.String("email", email),
		zap.String("user_id", user.ID),
	)

	return revokeUserRolePermission(ctx, c, user.ID, roleID, scopeResourceType, scopeResourceID, outputAnnotations)
}

// revokeUserRolePermission removes a single (role, scope resource) pair from a user's
// permissions. Segment has no endpoint to delete an individual permission, so the user's
// permissions are fetched, filtered and replaced. A user that no longer exists is
// reported as an already-revoked grant.
func revokeUserRolePermission(
	ctx context.Context,
	c *client.Client,
	userID string,
	roleID, scopeResourceType, scopeResourceID string,
	outputAnnotations *annotations.Annotations,
) error {
	userResp, rateLimit, err := c.GetUser(ctx, userID)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		if isNotFoundError(err) {
			outputAnnotations.Append(&v2.GrantAlreadyRevoked{})
			return nil
		}
		return fmt.Errorf("failed to get user permissions: %w", err)
	}

	filtered := filterOutPermission(userResp.Data.User.Permissions, roleID, scopeResourceType, scopeResourceID)

	_, rateLimit, err = c.ReplaceUserPermissions(ctx, userID, filtered)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return fmt.Errorf("failed to replace user permissions: %w", err)
	}

	return nil
}

// isInvitePending reports whether the email still has a pending workspace invitation.
// GET /invites only returns emails, so the list is paged through and matched
// case-insensitively.
func isInvitePending(
	ctx context.Context,
	c *client.Client,
	email string,
	outputAnnotations *annotations.Annotations,
) (bool, error) {
	cursor := ""
	for {
		response, rateLimit, err := c.ListInvites(ctx, cursor, client.DefaultPageSize)
		outputAnnotations.WithRateLimiting(rateLimit)
		if err != nil {
			return false, fmt.Errorf("failed to list invites: %w", err)
		}

		for _, pending := range response.Data.Invites {
			if strings.EqualFold(pending, email) {
				return true, nil
			}
		}

		next := response.Data.Pagination.Next
		if next == "" || next == cursor {
			return false, nil
		}
		cursor = next
	}
}

// findUserByEmail returns the workspace user with the given email, or nil when no user
// has it. Segment's list users endpoint takes no email filter, so the list is paged
// through and matched case-insensitively.
func findUserByEmail(
	ctx context.Context,
	c *client.Client,
	email string,
	outputAnnotations *annotations.Annotations,
) (*client.User, error) {
	cursor := ""
	for {
		response, rateLimit, err := c.ListUsers(ctx, cursor, client.DefaultPageSize)
		outputAnnotations.WithRateLimiting(rateLimit)
		if err != nil {
			return nil, fmt.Errorf("failed to list users: %w", err)
		}

		for _, user := range response.Data.Users {
			if strings.EqualFold(user.Email, email) {
				return &user, nil
			}
		}

		next := response.Data.Pagination.Next
		if next == "" || next == cursor {
			return nil, nil
		}
		cursor = next
	}
}
