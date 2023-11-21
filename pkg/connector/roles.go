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
)

const roleMembership = "member"

type roleBuilder struct {
	resourceType *v2.ResourceType
	client       *segment.Client
}

func (r *roleBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return r.resourceType
}

// Create a new connector resource for an Segment Role.
func roleResource(role *segment.Role, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"role_name":        role.Name,
		"role_id":          role.ID,
		"role_description": role.Description,
	}

	roleTraitOptions := []rs.RoleTraitOption{
		rs.WithRoleProfile(profile),
	}

	resource, err := rs.NewRoleResource(
		role.Name,
		roleResourceType,
		role.ID,
		roleTraitOptions,
		rs.WithParentResourceID(parentResourceID),
	)

	if err != nil {
		return nil, err
	}

	return resource, nil
}

func (r *roleBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	bag, page, err := parsePageToken(pToken.Token, &v2.ResourceId{ResourceType: roleResourceType.Id})
	if err != nil {
		return nil, "", nil, err
	}

	roles, nextCursor, err := r.client.ListRoles(ctx, page)
	if err != nil {
		return nil, "", nil, err
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Resource
	for _, role := range roles {
		roleCopy := role
		rr, err := roleResource(&roleCopy, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}

		rv = append(rv, rr)
	}

	return rv, pageToken, nil, nil
}

func (r *roleBuilder) Entitlements(ctx context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var rv []*v2.Entitlement
	assignmentOptions := []ent.EntitlementOption{
		ent.WithGrantableTo(userResourceType, groupResourceType),
		ent.WithDisplayName(fmt.Sprintf("%s Role %s", resource.DisplayName, roleMembership)),
		ent.WithDescription(fmt.Sprintf("Member of %s Segment role", resource.DisplayName)),
	}

	rv = append(rv, ent.NewAssignmentEntitlement(
		resource,
		roleMembership,
		assignmentOptions...,
	))
	return rv, "", nil, nil
}

func (r *roleBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	bag, page, err := parsePageToken(pToken.Token, &v2.ResourceId{ResourceType: roleResourceType.Id})
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Grant
	switch bag.ResourceTypeID() {
	case roleResourceType.Id:
		bag.Pop()
		bag.Push(pagination.PageState{
			ResourceTypeID: userResourceType.Id,
		})
		bag.Push(pagination.PageState{
			ResourceTypeID: groupResourceType.Id,
		})

	case userResourceType.Id:
		users, nextCursor, err := r.client.ListUsers(ctx, page)
		if err != nil {
			return nil, "", nil, err
		}

		paginationErr := bag.Next(nextCursor)
		if paginationErr != nil {
			return nil, "", nil, paginationErr
		}

		for _, user := range users {
			userCopy := user
			for _, userPermission := range user.Permissions {
				if userPermission.RoleID == resource.Id.Resource {
					ur, err := userResource(&userCopy, resource.Id)
					if err != nil {
						return nil, "", nil, fmt.Errorf("error creating user resource for role %s: %w", resource.Id.Resource, err)
					}
					gr := grant.NewGrant(resource, roleMembership, ur.Id)
					rv = append(rv, gr)
				}
			}
		}
	case groupResourceType.Id:
		groups, nextCursor, err := r.client.ListGroups(ctx, page)
		if err != nil {
			return nil, "", nil, err
		}

		paginationErr := bag.Next(nextCursor)
		if paginationErr != nil {
			return nil, "", nil, paginationErr
		}

		for _, group := range groups {
			groupCopy := group

			for _, groupPermission := range group.Permissions {
				if groupPermission.RoleID == resource.Id.Resource {
					gRes, err := groupResource(&groupCopy, resource.Id)
					if err != nil {
						return nil, "", nil, fmt.Errorf("error creating group resource for role %s: %w", resource.Id.Resource, err)
					}

					expandAnnos := &v2.GrantExpandable{
						EntitlementIds: []string{
							fmt.Sprintf("group:%s:member", groupCopy.ID),
						},
						Shallow: true,
					}
					gr := grant.NewGrant(resource, roleMembership, gRes.Id, grant.WithAnnotation(expandAnnos))
					rv = append(rv, gr)
				}
			}
		}
	default:
		return nil, "", nil, fmt.Errorf("unexpected resource type while fetching grants for a role")
	}

	pageToken, err := bag.Marshal()
	if err != nil {
		return nil, "", nil, err
	}

	return rv, pageToken, nil, nil
}

func newRoleBuilder(client *segment.Client) *roleBuilder {
	return &roleBuilder{
		resourceType: roleResourceType,
		client:       client,
	}
}
