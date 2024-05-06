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

const workspaceMembership = "member"

type workspaceBuilder struct {
	resourceType *v2.ResourceType
	client       *segment.Client
}

func (w *workspaceBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return w.resourceType
}

// Create a new connector resource for a Segment workspace.
func workspaceResource(workspace *segment.Workspace) (*v2.Resource, error) {
	ret, err := rs.NewResource(
		workspace.Name,
		workspaceResourceType,
		workspace.ID,
		rs.WithAnnotation(
			&v2.ChildResourceType{ResourceTypeId: userResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: groupResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: roleResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: functionResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: sourceResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: warehouseResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: spaceResourceType.Id},
		),
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (w *workspaceBuilder) List(ctx context.Context, _ *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	workspace, err := w.client.GetWorkspace(ctx)
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Resource
	ur, err := workspaceResource(workspace)
	if err != nil {
		return nil, "", nil, err
	}
	rv = append(rv, ur)

	return rv, "", nil, nil
}

func (w *workspaceBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var rv []*v2.Entitlement
	assignmentOptions := []ent.EntitlementOption{
		ent.WithGrantableTo(userResourceType),
		ent.WithDisplayName(fmt.Sprintf("%s Workspace %s", resource.DisplayName, workspaceMembership)),
		ent.WithDescription(fmt.Sprintf("Member of %s Segement workspace", resource.DisplayName)),
	}

	rv = append(rv, ent.NewAssignmentEntitlement(
		resource,
		workspaceMembership,
		assignmentOptions...,
	))

	return rv, "", nil, nil
}

func (w *workspaceBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	bag, page, err := parsePageToken(pToken.Token, resource.Id)
	if err != nil {
		return nil, "", nil, err
	}

	users, nextToken, err := w.client.ListUsers(ctx, page)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to list workspace members: %w", err)
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
			return nil, "", nil, fmt.Errorf("error creating workspace user %s: %w", resource.Id.Resource, err)
		}
		rv = append(rv, grant.NewGrant(
			resource,
			workspaceMembership,
			ur,
		))
	}

	return rv, pageToken, nil, nil
}

func newWorkspaceBuilder(client *segment.Client) *workspaceBuilder {
	return &workspaceBuilder{
		resourceType: workspaceResourceType,
		client:       client,
	}
}
