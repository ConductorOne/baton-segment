package connector

import (
	"context"
	"errors"
	"fmt"
	"strings"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/cli"
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
			ent.WithGrantableTo(userResourceType),
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

	if principal.Id.ResourceType != userResourceType.Id {
		return nil, nil, fmt.Errorf("only users can be granted role assignments, got %s", principal.Id.ResourceType)
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
func revokeRoleEntitlement(ctx context.Context, c *client.Client, grant *v2.Grant) (annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)
	outputAnnotations := annotations.New()

	principal := grant.Principal
	if principal.Id.ResourceType != userResourceType.Id {
		return nil, fmt.Errorf("only users can have role assignments revoked, got %s", principal.Id.ResourceType)
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

	// Fetch current permissions for the user
	userResp, rl, err := c.GetUser(ctx, principal.Id.Resource)
	outputAnnotations.WithRateLimiting(rl)
	if err != nil {
		if isNotFoundError(err) {
			outputAnnotations.Append(&v2.GrantAlreadyRevoked{})
			return outputAnnotations, nil
		}
		return outputAnnotations, fmt.Errorf("failed to get user permissions: %w", err)
	}
	currentPermissions := userResp.Data.User.Permissions

	// Filter out the target permission and replace
	filteredPermissions := filterOutPermission(currentPermissions, role.ID, scopeResourceType, scopeResourceID)

	_, rl, err = c.ReplaceUserPermissions(ctx, principal.Id.Resource, filteredPermissions)
	outputAnnotations.WithRateLimiting(rl)
	if err != nil {
		return outputAnnotations, fmt.Errorf("failed to replace user permissions: %w", err)
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

// skipCrossTypeGrants is the set of cross-type target resource types excluded
// from this sync, precomputed once so builders take booleans rather than the
// whole *cli.ConnectorOpts.
type skipCrossTypeGrants map[string]bool

// skip reports whether grants targeting resourceTypeID should be suppressed.
func (s skipCrossTypeGrants) skip(resourceTypeID string) bool { return s[resourceTypeID] }

// all reports whether every cross-type target is excluded. Only safe to act on
// for builders whose grants are all cross-type (userBuilder); groupBuilder also
// emits its own member grants, so it uses this to skip the group-roles fetch
// rather than the whole grants pass.
func (s skipCrossTypeGrants) all() bool {
	for _, id := range crossTypeGrantTargets {
		if !s[id] {
			return false
		}
	}
	return true
}

// crossTypeGrantTargets are the resource types that user/group grants can
// reference, derived from a permission's scope.
var crossTypeGrantTargets = []string{
	roleResourceType.Id,
	sourceResourceType.Id,
	warehouseResourceType.Id,
	functionResourceType.Id,
	spaceResourceType.Id,
}

// newSkipCrossTypeGrants precomputes the skip decision for every target type.
// nil cliOpts means no filter, so nothing is skipped.
func newSkipCrossTypeGrants(cliOpts *cli.ConnectorOpts) skipCrossTypeGrants {
	out := make(skipCrossTypeGrants, len(crossTypeGrantTargets))
	for _, id := range crossTypeGrantTargets {
		out[id] = cliOpts != nil && !cliOpts.WillSyncResourceType(id)
	}
	return out
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
