package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	gr "github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-segment/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

const (
	groupMemberEntitlement = "member"
)

// getEmailFromResource extracts the primary email from a user resource's UserTrait.
func getEmailFromResource(resource *v2.Resource) (string, error) {
	userTrait, err := rs.GetUserTrait(resource)
	if err != nil {
		return "", fmt.Errorf("failed to get user trait: %w", err)
	}

	emails := userTrait.GetEmails()
	if len(emails) == 0 {
		return "", fmt.Errorf("user has no email addresses")
	}

	// Return the primary email, or the first one if none is marked primary
	for _, email := range emails {
		if email.GetIsPrimary() {
			return email.GetAddress(), nil
		}
	}

	return emails[0].GetAddress(), nil
}

type groupBuilder struct {
	client *client.Client
}

func (b *groupBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return groupResourceType
}

// List returns all the groups from the Segment workspace.
func (b *groupBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, opts rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)

	bag, err := parsePageToken(opts.PageToken.Token, &v2.ResourceId{ResourceType: groupResourceType.Id})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse page token: %w", err)
	}

	pageToken := bag.PageToken()

	outputAnnotations := annotations.New()
	response, rateLimit, err := b.client.ListGroups(ctx, pageToken, client.DefaultPageSize)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to list groups: %w", err)
	}

	var resources []*v2.Resource
	for _, group := range response.Data.UserGroups {
		r, err := groupResource(&group, parentResourceID)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to create group resource: %w", err)
		}
		resources = append(resources, r)
	}

	nextToken, err := bag.NextToken(response.Data.Pagination.Next)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to create next page token: %w", err)
	}

	l.Debug("listed groups", zap.Int("count", len(resources)), zap.String("next_token", nextToken))

	return resources, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: outputAnnotations}, nil
}

// groupResource creates a v2.Resource from a Segment group.
func groupResource(group *client.Group, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"id":           group.ID,
		"member_count": group.MemberCount,
	}

	groupTraitOptions := []rs.GroupTraitOption{
		rs.WithGroupProfile(profile),
	}

	return rs.NewGroupResource(
		group.Name,
		groupResourceType,
		group.ID,
		groupTraitOptions,
		rs.WithParentResourceID(parentResourceID),
	)
}

// Entitlements returns no dynamic entitlements for groups.
// The member entitlement is handled as a static entitlement.
func (b *groupBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	return []*v2.Entitlement{}, nil, nil
}

// StaticEntitlements returns the member entitlement template for all groups.
// The SDK creates one entitlement per group resource using this template.
func (b *groupBuilder) StaticEntitlements(_ context.Context, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	entitlements := []*v2.Entitlement{
		ent.NewAssignmentEntitlement(
			nil,
			groupMemberEntitlement,
			ent.WithDisplayName("Member"),
			ent.WithDescription("Member of the group"),
			ent.WithGrantableTo(userResourceType),
		),
	}

	return entitlements, nil, nil
}

// Grants returns the membership grants for a group.
func (b *groupBuilder) Grants(ctx context.Context, resource *v2.Resource, opts rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)
	groupID := resource.Id.Resource

	// Handle pagination for multi-phase grants (members + role assignments)
	bag := &pagination.Bag{}
	if opts.PageToken.Token != "" {
		if err := bag.Unmarshal(opts.PageToken.Token); err != nil {
			return nil, nil, fmt.Errorf("failed to parse page token: %w", err)
		}
	}

	currentState := bag.Current()
	// Initialize pagination state on first call (bag is empty)
	if currentState == nil {
		// Push in reverse order since it's a stack (last pushed = first processed)
		bag.Push(pagination.PageState{ResourceTypeID: "group-roles", ResourceID: groupID})
		bag.Push(pagination.PageState{ResourceTypeID: "group-members", ResourceID: groupID})
		currentState = bag.Current()
	}

	var grants []*v2.Grant
	var nextToken string
	outputAnnotations := annotations.New()

	switch currentState.ResourceTypeID {
	case "group-members":
		// Fetch group members
		response, rateLimit, err := b.client.ListGroupUsers(ctx, groupID, bag.PageToken(), client.DefaultPageSize)
		outputAnnotations.WithRateLimiting(rateLimit)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to list group users: %w", err)
		}

		for _, user := range response.Data.Users {
			grant := gr.NewGrant(resource, groupMemberEntitlement, &v2.ResourceId{
				ResourceType: userResourceType.Id,
				Resource:     user.ID,
			})
			grants = append(grants, grant)
		}

		nextToken, err = bag.NextToken(response.Data.Pagination.Next)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to create next page token: %w", err)
		}

		l.Debug("listed group member grants", zap.String("group_id", groupID), zap.Int("count", len(grants)))

	case "group-roles":
		// Fetch group details to get role assignments
		groupResp, rateLimit, err := b.client.GetGroup(ctx, groupID)
		outputAnnotations.WithRateLimiting(rateLimit)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to get group details: %w", err)
		}

		// Create grants for group -> role assignments.
		// Mark as expandable so the SDK expands group members into individual grants.
		// Segment roles are additive: group members inherit all roles assigned to the group.
		expandable := gr.WithAnnotation(&v2.GrantExpandable{
			EntitlementIds: []string{
				fmt.Sprintf("group:%s:%s", resource.Id.Resource, groupMemberEntitlement),
			},
			Shallow: true,
			ResourceTypeIds: []string{
				userResourceType.Id,
			},
		})

		for _, perm := range groupResp.Data.UserGroup.Permissions {
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
					// Mirrors users.go to avoid dangling grants against removed workspace role entitlements.
					roleRes := &v2.Resource{
						Id: &v2.ResourceId{
							ResourceType: roleResourceType.Id,
							Resource:     perm.RoleID,
						},
					}
					grant = gr.NewGrant(roleRes, roleMembership, resource.Id, expandable)
				} else {
					// Source/function/etc-scoped permissions: grant on scope resource with snake_case slug.
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
					grant = gr.NewGrant(scopeResource, roleSlug, resource.Id, expandable)
				}
				grants = append(grants, grant)
			}
		}

		// Pop this state, move to next (empty string means done)
		nextToken, err = bag.NextToken("")
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to create next page token: %w", err)
		}

		l.Debug("listed group role grants", zap.String("group_id", groupID), zap.Int("count", len(grants)))
	}

	return grants, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: outputAnnotations}, nil
}

// Grant adds a user to the group.
func (b *groupBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) ([]*v2.Grant, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	if principal.Id.ResourceType != userResourceType.Id {
		return nil, nil, fmt.Errorf("only users can be granted group membership")
	}

	// Get email from the principal resource's UserTrait
	email, err := getEmailFromResource(principal)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get email from principal: %w", err)
	}

	outputAnnotations := annotations.New()
	groupID := entitlement.Resource.Id.Resource
	userID := principal.Id.Resource

	l.Debug("granting group membership",
		zap.String("group_id", groupID),
		zap.String("user_id", userID),
		zap.String("email", email),
	)

	_, rateLimit, err := b.client.AddUsersToGroup(ctx, groupID, []string{email})
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return nil, outputAnnotations, fmt.Errorf("failed to add user to group: %w", err)
	}

	grant := gr.NewGrant(entitlement.Resource, groupMemberEntitlement, principal.Id)

	l.Debug("group membership granted successfully",
		zap.String("group_id", groupID),
		zap.String("user_id", userID),
	)

	return []*v2.Grant{grant}, outputAnnotations, nil
}

// Revoke removes a user from the group.
func (b *groupBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	groupID := grant.Entitlement.Resource.Id.Resource
	userID := grant.Principal.Id.Resource

	// Get email from the principal resource's UserTrait
	email, err := getEmailFromResource(grant.Principal)
	if err != nil {
		return nil, fmt.Errorf("failed to get email from principal: %w", err)
	}

	l.Debug("revoking group membership",
		zap.String("group_id", groupID),
		zap.String("user_id", userID),
		zap.String("email", email),
	)

	outputAnnotations := annotations.New()
	rateLimit, err := b.client.RemoveUsersFromGroup(ctx, groupID, []string{email})
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return outputAnnotations, fmt.Errorf("failed to remove user from group: %w", err)
	}

	l.Debug("group membership revoked successfully",
		zap.String("group_id", groupID),
		zap.String("user_id", userID),
	)

	return outputAnnotations, nil
}

func newGroupBuilder(c *client.Client) *groupBuilder {
	return &groupBuilder{client: c}
}
