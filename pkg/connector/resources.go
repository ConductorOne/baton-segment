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

type roleResourceBuilder struct {
	resourceType *v2.ResourceType
	client       *segment.Client
}

func (r *roleResourceBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return r.resourceType
}

// Create a new connector resource for an Segment Resource.
func resourceRoleResource(roleResource *segment.Resource, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"resource_name": roleResource.Type,
		"resource_id":   roleResource.ID,
	}

	roleTraitOptions := []rs.RoleTraitOption{
		rs.WithRoleProfile(profile),
	}

	resource, err := rs.NewRoleResource(
		roleResource.Type,
		roleResourceType,
		roleResource.ID,
		roleTraitOptions,
		rs.WithParentResourceID(parentResourceID),
	)

	if err != nil {
		return nil, err
	}

	return resource, nil
}

func (r *roleResourceBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	bag, page, err := parsePageToken(pToken.Token, &v2.ResourceId{ResourceType: userResourceType.Id})
	if err != nil {
		return nil, "", nil, err
	}

	// the resources can be found under user.Permissions.Resources.
	users, nextCursor, err := r.client.ListUsers(ctx, page)
	if err != nil {
		return nil, "", nil, err
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Resource
	for _, user := range users {
		for _, userPermission := range user.Permissions {
			for _, roleResource := range userPermission.Resources {
				roleResourceCopy := roleResource
				rr, err := resourceRoleResource(&roleResourceCopy, parentResourceID)
				if err != nil {
					return nil, "", nil, err
				}

				rv = append(rv, rr)
			}
		}
	}

	return rv, pageToken, nil, nil
}

func (r *roleResourceBuilder) Entitlements(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	bag, page, err := parsePageToken(pToken.Token, &v2.ResourceId{ResourceType: roleResourceType.Id})
	if err != nil {
		return nil, "", nil, err
	}

	// roles act as entitlements on the resource.
	roles, nextCursor, err := r.client.ListRoles(ctx, page)
	if err != nil {
		return nil, "", nil, err
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Entitlement
	for _, role := range roles {
		permissionOptions := []ent.EntitlementOption{
			ent.WithGrantableTo(userResourceType, groupResourceType),
			ent.WithDisplayName(fmt.Sprintf("%s resource %s", resource.DisplayName, role.Name)),
			ent.WithDescription(fmt.Sprintf("%s on %s resource with id: %s", role.Name, resource.DisplayName, resource.Id.Resource)),
		}

		rv = append(rv, ent.NewPermissionEntitlement(
			resource,
			role.Name,
			permissionOptions...,
		))
	}
	return rv, pageToken, nil, nil
}

func (r *roleResourceBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	bag, page, err := parsePageToken(pToken.Token, &v2.ResourceId{ResourceType: resourceResourceType.Id})
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Grant
	switch bag.ResourceTypeID() {
	case resourceResourceType.Id:
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
				for _, roleResource := range userPermission.Resources {
					if roleResource.ID == resource.Id.Resource {
						ur, err := userResource(&userCopy, resource.Id)
						if err != nil {
							return nil, "", nil, fmt.Errorf("error creating user resource for resource %s: %w", resource.Id.Resource, err)
						}
						gr := grant.NewGrant(resource, userPermission.RoleName, ur.Id)
						rv = append(rv, gr)
					}
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
			for _, groupPermission := range group.Permissions {
				for _, roleResource := range groupPermission.Resources {
					if roleResource.ID == resource.Id.Resource {
						groupCopy := group
						gRes, err := groupResource(&groupCopy, resource.Id)
						if err != nil {
							return nil, "", nil, fmt.Errorf("error creating group resource for resource %s: %w", resource.Id.Resource, err)
						}

						expandAnnos := &v2.GrantExpandable{
							EntitlementIds: []string{
								fmt.Sprintf("group:%s:member", groupCopy.ID),
							},
							Shallow: true,
						}

						gr := grant.NewGrant(resource, groupPermission.RoleName, gRes.Id, grant.WithAnnotation(expandAnnos))
						rv = append(rv, gr)
					}
				}
			}
		}
	default:
		return nil, "", nil, fmt.Errorf("unexpected resource type while fetching grants for a resource role resource")
	}

	pageToken, err := bag.Marshal()
	if err != nil {
		return nil, "", nil, err
	}

	return rv, pageToken, nil, nil
}

func newRoleResourceBuilder(client *segment.Client) *roleResourceBuilder {
	return &roleResourceBuilder{
		resourceType: resourceResourceType,
		client:       client,
	}
}
