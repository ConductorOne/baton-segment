package connector

import (
	"context"
	"fmt"
	"strings"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-segment/pkg/segment"
)

type spaceResourceBuilder struct {
	resourceType *v2.ResourceType
	client       *segment.Client
}

func (s *spaceResourceBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return s.resourceType
}

// Create a new connector resource for an Segment Space.
func spaceResource(space *segment.Space, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	resource, err := rs.NewResource(
		space.Name,
		spaceResourceType,
		space.ID,
		rs.WithParentResourceID(parentResourceID),
	)

	if err != nil {
		return nil, err
	}

	return resource, nil
}

func (s *spaceResourceBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	bag, page, err := parsePageToken(pToken.Token, &v2.ResourceId{ResourceType: spaceResourceType.Id})
	if err != nil {
		return nil, "", nil, err
	}

	spaces, nextCursor, err := s.client.ListSpaces(ctx, page)
	if err != nil {
		return nil, "", nil, err
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Resource
	for _, space := range spaces {
		spaceCopy := space
		sr, err := spaceResource(&spaceCopy, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}

		rv = append(rv, sr)
	}

	return rv, pageToken, nil, nil
}

func (s *spaceResourceBuilder) Entitlements(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	bag, page, err := parsePageToken(pToken.Token, &v2.ResourceId{ResourceType: roleResourceType.Id})
	if err != nil {
		return nil, "", nil, err
	}

	roles, nextCursor, err := s.client.ListRoles(ctx, page)
	if err != nil {
		return nil, "", nil, err
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Entitlement
	for _, role := range roles {
		// all role names for Spaces contain "Engage" instead of "Space"
		if strings.Contains(role.Name, "Engage") {
			entitlement := createEntitlement(role, resource)
			rv = append(rv, entitlement)
		}
	}

	return rv, pageToken, nil, nil
}

// We do the grants on User and Group level.
func (s *spaceResourceBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (s *spaceResourceBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	roleID, resourceType := getRoleIdAndResourceType(entitlement)
	resourceID := entitlement.Resource.Id.Resource
	permissions, err := grantPermissions(ctx, s.client, principal, roleID, resourceType, resourceID)
	if err != nil {
		return nil, err
	}

	err = s.client.UpdatePermissions(ctx, principal.Id.Resource, principal.Id.ResourceType, permissions)
	if err != nil {
		return nil, fmt.Errorf(
			"baton-segment: failed to add permission to %s %s for space resource %s: %w",
			principal.Id.ResourceType,
			principal.DisplayName,
			entitlement.Resource.DisplayName,
			err,
		)
	}

	return nil, nil
}

func (s *spaceResourceBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	entitlement := grant.Entitlement
	principal := grant.Principal
	roleID, _ := getRoleIdAndResourceType(entitlement)

	permissions, err := revokePermissions(ctx, s.client, principal, roleID)
	if err != nil {
		return nil, err
	}

	err = s.client.UpdatePermissions(ctx, principal.Id.Resource, principal.Id.ResourceType, permissions)
	if err != nil {
		return nil, fmt.Errorf(
			"baton-segment: failed to remove permission from %s %s for space resource %s: %w",
			principal.Id.ResourceType,
			principal.DisplayName,
			entitlement.Resource.DisplayName,
			err,
		)
	}

	return nil, nil
}

func newSpaceBuilder(client *segment.Client) *spaceResourceBuilder {
	return &spaceResourceBuilder{
		resourceType: spaceResourceType,
		client:       client,
	}
}
