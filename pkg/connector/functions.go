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

type functionResourceBuilder struct {
	resourceType *v2.ResourceType
	client       *segment.Client
}

func (f *functionResourceBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return f.resourceType
}

// Create a new connector resource for an Segment Function.
func functionResource(function *segment.Function, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	resource, err := rs.NewResource(
		function.DisplayName,
		functionResourceType,
		function.ID,
		rs.WithParentResourceID(parentResourceID),
	)
	if err != nil {
		return nil, err
	}

	return resource, nil
}

func (f *functionResourceBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	// There are 3 types of functions, we need to fetch all of them
	functionTypes := []string{"DESTINATION", "INSERT_DESTINATION", "SOURCE"}
	var cursor string
	var allFunctions []segment.Function

	for _, t := range functionTypes {
		cursor = ""
		for {
			// Fetch data for the current type
			functions, nextCursor, err := f.client.ListFunctions(ctx, cursor, t)
			allFunctions = append(allFunctions, functions...)
			if err != nil {
				fmt.Printf("Error fetching data for type %s: %v\n", t, err)
				break
			}

			if nextCursor != "" {
				cursor = nextCursor
			} else {
				// No more items for the current type, move to the next one
				break
			}
		}
	}

	var rv []*v2.Resource
	for _, fn := range allFunctions {
		fr, err := functionResource(&fn, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}

		rv = append(rv, fr)
	}

	return rv, "", nil, nil
}

func (f *functionResourceBuilder) Entitlements(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	bag, page, err := parsePageToken(pToken.Token, &v2.ResourceId{ResourceType: roleResourceType.Id})
	if err != nil {
		return nil, "", nil, err
	}

	roles, nextCursor, err := f.client.ListRoles(ctx, page)
	if err != nil {
		return nil, "", nil, err
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Entitlement
	for _, role := range roles {
		if strings.Contains(role.Name, "Function") {
			entitlement := createEntitlement(role, resource)
			rv = append(rv, entitlement)
		}
	}

	return rv, pageToken, nil, nil
}

func (f *functionResourceBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (f *functionResourceBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	roleID, resourceType := getRoleIdAndResourceType(entitlement)
	resourceID := entitlement.Resource.Id.Resource
	permissions, err := grantPermissions(ctx, f.client, principal, roleID, resourceType, resourceID)
	if err != nil {
		return nil, err
	}

	err = f.client.UpdatePermissions(ctx, principal.Id.Resource, principal.Id.ResourceType, permissions)
	if err != nil {
		return nil, fmt.Errorf(
			"baton-segment: failed to add permission to %s %s for function resource %s: %w",
			principal.Id.ResourceType,
			principal.DisplayName,
			entitlement.Resource.DisplayName,
			err,
		)
	}

	return nil, nil
}

func (f *functionResourceBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	entitlement := grant.Entitlement
	principal := grant.Principal
	roleID, _ := getRoleIdAndResourceType(entitlement)

	newPermissions, err := revokePermissions(ctx, f.client, principal, roleID)
	if err != nil {
		return nil, err
	}

	err = f.client.UpdatePermissions(ctx, principal.Id.Resource, principal.Id.ResourceType, newPermissions)
	if err != nil {
		return nil, fmt.Errorf(
			"baton-segment: failed to remove permission from %s %s for function resource %s: %w",
			principal.Id.ResourceType,
			principal.DisplayName,
			entitlement.Resource.DisplayName,
			err,
		)
	}

	return nil, nil
}

func newFunctionBuilder(client *segment.Client) *functionResourceBuilder {
	return &functionResourceBuilder{
		resourceType: functionResourceType,
		client:       client,
	}
}
