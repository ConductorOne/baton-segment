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

type sourceResourceBuilder struct {
	resourceType *v2.ResourceType
	client       *segment.Client
}

func (s *sourceResourceBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return s.resourceType
}

// Create a new connector resource for an Segment Source.
func sourceResource(source *segment.Source, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	resource, err := rs.NewResource(
		source.Name,
		sourceResourceType,
		source.ID,
		rs.WithParentResourceID(parentResourceID),
	)

	if err != nil {
		return nil, err
	}

	return resource, nil
}

func (s *sourceResourceBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	bag, page, err := parsePageToken(pToken.Token, &v2.ResourceId{ResourceType: sourceResourceType.Id})
	if err != nil {
		return nil, "", nil, err
	}

	sources, nextCursor, err := s.client.ListSources(ctx, page)
	if err != nil {
		return nil, "", nil, err
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Resource
	for _, source := range sources {
		sourceCopy := source
		sr, err := sourceResource(&sourceCopy, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}

		rv = append(rv, sr)
	}

	return rv, pageToken, nil, nil
}

func (s *sourceResourceBuilder) Entitlements(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
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
		if strings.Contains(role.Name, "Source") {
			entitlement := createEntitlement(role, resource)
			rv = append(rv, entitlement)
		}
	}

	return rv, pageToken, nil, nil
}

func (s *sourceResourceBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (s *sourceResourceBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	roleID, resourceType := getRoleIdAndResourceType(entitlement)
	resourceID := entitlement.Resource.Id.Resource
	permissions, err := grantPermissions(ctx, s.client, principal, roleID, resourceType, resourceID)
	if err != nil {
		return nil, err
	}

	err = s.client.UpdatePermissions(ctx, principal.Id.Resource, principal.Id.ResourceType, permissions)
	if err != nil {
		return nil, fmt.Errorf(
			"baton-segment: failed to add permission to %s %s for source resource %s: %w",
			principal.Id.ResourceType,
			principal.DisplayName,
			entitlement.Resource.DisplayName,
			err,
		)
	}

	return nil, nil
}

func (s *sourceResourceBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
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
			"baton-segment: failed to remove permission from %s %s for source resource %s: %w",
			principal.Id.ResourceType,
			principal.DisplayName,
			entitlement.Resource.DisplayName,
			err,
		)
	}

	return nil, nil
}

func newSourceBuilder(client *segment.Client) *sourceResourceBuilder {
	return &sourceResourceBuilder{
		resourceType: sourceResourceType,
		client:       client,
	}
}
