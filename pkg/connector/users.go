package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/helpers"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	grant "github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-segment/pkg/segment"
	"github.com/iancoleman/strcase"
)

const (
	functionType  = "FUNCTION"
	sourceType    = "SOURCE"
	warehouseType = "WAREHOUSE"
	spaceType     = "SPACE"
	workspaceType = "WORKSPACE"
)

type userBuilder struct {
	resourceType *v2.ResourceType
	client       *segment.Client
}

func (u *userBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return u.resourceType
}

// Create a new connector resource for a Segment user.
func userResource(user *segment.User, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	firstName, lastName := helpers.SplitFullName(user.Name)
	profile := map[string]interface{}{
		"first_name": firstName,
		"last_name":  lastName,
		"login":      user.Email,
		"user_id":    user.ID,
	}

	userTraitOptions := []rs.UserTraitOption{
		rs.WithUserProfile(profile),
		rs.WithEmail(user.Email, true),
		rs.WithStatus(v2.UserTrait_Status_STATUS_ENABLED),
	}

	ret, err := rs.NewUserResource(
		user.Name,
		userResourceType,
		user.ID,
		userTraitOptions,
		rs.WithParentResourceID(parentResourceID),
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (u *userBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	bag, page, err := parsePageToken(pToken.Token, &v2.ResourceId{ResourceType: userResourceType.Id})
	if err != nil {
		return nil, "", nil, err
	}

	users, nextCursor, err := u.client.ListUsers(ctx, page)
	if err != nil {
		return nil, "", nil, err
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Resource
	for _, user := range users {
		userCopy := user
		ur, err := userResource(&userCopy, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, ur)
	}

	return rv, pageToken, nil, nil
}

// Entitlements always returns an empty slice for users.
func (u *userBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

// usually grants are not implemented on the user, but due to the way the segment API is structured, it's easier to implement it here
func (u *userBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	user, err := u.client.GetUser(ctx, resource.Id.Resource)
	if err != nil {
		return nil, "", nil, err
	}

	ur, err := userResource(user, resource.ParentResourceId)
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Grant
	for _, p := range user.Permissions {
		var role segment.Role
		role.ID = p.RoleID
		role.Name = p.RoleName
		rr, err := roleResource(&role, resource.ParentResourceId)
		if err != nil {
			return nil, "", nil, fmt.Errorf("error creating role resource for user permissions")
		}

		// grant user permissions on resources
		for _, r := range p.Resources {
			var roleResource segment.Resource
			roleResource.ID = r.ID
			roleResource.Type = r.Type
			roleEntitlement := strcase.ToSnake(role.Name)

			if r.Type == workspaceType {
				rv = append(rv, grant.NewGrant(rr, roleMembership, ur.Id))
				continue
			}

			resource, err := baseResource(roleResource, resource.ParentResourceId)
			if err != nil {
				return nil, "", nil, fmt.Errorf("error creating %s resource", r.Type)
			}

			rv = append(rv, grant.NewGrant(resource, roleEntitlement, ur.Id))
		}
	}

	return rv, "", nil, nil
}

func newUserBuilder(client *segment.Client) *userBuilder {
	return &userBuilder{
		resourceType: userResourceType,
		client:       client,
	}
}
