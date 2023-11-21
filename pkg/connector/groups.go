package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	grant "github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-segment/pkg/segment"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

type groupBuilder struct {
	resourceType *v2.ResourceType
	client       *segment.Client
}

const groupMembership = "member"

func (g *groupBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return g.resourceType
}

// Create a new connector resource for a Segment user group.
func groupResource(group *segment.Group, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"group_name": group.Name,
		"group_id":   group.ID,
	}

	groupTraitOptions := []rs.GroupTraitOption{
		rs.WithGroupProfile(profile),
	}

	ret, err := rs.NewGroupResource(
		group.Name,
		groupResourceType,
		group.ID,
		groupTraitOptions,
		rs.WithParentResourceID(parentResourceID),
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// List returns all the user groups from the database as resource objects.
// Groups include a Group because they are the 'shape' of a standard group.
func (g *groupBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	bag, page, err := parsePageToken(pToken.Token, &v2.ResourceId{ResourceType: groupResourceType.Id})
	if err != nil {
		return nil, "", nil, err
	}

	groups, nextCursor, err := g.client.ListGroups(ctx, page)
	if err != nil {
		return nil, "", nil, err
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Resource
	for _, group := range groups {
		groupCopy := group
		gr, err := groupResource(&groupCopy, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, gr)
	}

	return rv, pageToken, nil, nil
}

func (g *groupBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var rv []*v2.Entitlement
	assignmentOptions := []ent.EntitlementOption{
		ent.WithGrantableTo(userResourceType),
		ent.WithDisplayName(fmt.Sprintf("%s group %s", resource.DisplayName, groupMembership)),
		ent.WithDescription(fmt.Sprintf("Member of %s Segment group", resource.DisplayName)),
	}

	rv = append(rv, ent.NewAssignmentEntitlement(
		resource,
		groupMembership,
		assignmentOptions...,
	))

	return rv, "", nil, nil
}

func (g *groupBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	bag, page, err := parsePageToken(pToken.Token, &v2.ResourceId{ResourceType: userResourceType.Id})
	if err != nil {
		return nil, "", nil, err
	}

	users, nextToken, err := g.client.ListGroupMembers(ctx, resource.Id.Resource, page)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to list group members: %w", err)
	}

	pageToken, err := bag.NextToken(nextToken)
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Grant
	for _, user := range users {
		userCopy := user
		ur, err := userResource(&userCopy, resource.Id)
		if err != nil {
			return nil, "", nil, fmt.Errorf("error creating user resource for group %s: %w", resource.Id.Resource, err)
		}

		gr := grant.NewGrant(resource, groupMembership, ur.Id)
		rv = append(rv, gr)
	}

	return rv, pageToken, nil, nil
}

func (g *groupBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	if principal.Id.ResourceType != userResourceType.Id {
		l.Warn(
			"baton-segment: only users can be granted group membership",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("baton-segment: only users can be granted group membership")
	}

	userTrait, err := rs.GetUserTrait(principal)
	if err != nil {
		return nil, err
	}

	userEmail, ok := rs.GetProfileStringValue(userTrait.Profile, "login")
	if !ok {
		return nil, err
	}

	err = g.client.AddGroupMembers(ctx, entitlement.Resource.Id.Resource, userEmail)
	if err != nil {
		return nil, fmt.Errorf("baton-segment: failed to add user to group: %w", err)
	}

	return nil, nil
}

func (g *groupBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	entitlement := grant.Entitlement
	principal := grant.Principal

	if principal.Id.ResourceType != userResourceType.Id {
		l.Warn(
			"baton-segment: only users can have group membership revoked",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("baton-segment: only users can have group membership revoked")
	}

	userTrait, err := rs.GetUserTrait(principal)
	if err != nil {
		return nil, err
	}

	userEmail, ok := rs.GetProfileStringValue(userTrait.Profile, "login")
	if !ok {
		return nil, err
	}

	err = g.client.RemoveGroupMember(ctx, entitlement.Resource.Id.Resource, userEmail)
	if err != nil {
		return nil, fmt.Errorf("baton-segment: failed to revoke group membership for user: %s: %w", principal.Id, err)
	}

	return nil, nil
}

func newGroupBuilder(client *segment.Client) *groupBuilder {
	return &groupBuilder{
		resourceType: groupResourceType,
		client:       client,
	}
}
