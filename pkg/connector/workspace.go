package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	gr "github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-segment/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

const (
	workspaceMemberEntitlement = "member"
)

type workspaceBuilder struct {
	client *client.Client
}

func (b *workspaceBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return workspaceResourceType
}

// List returns the workspace resource (there's only one per token).
func (b *workspaceBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, opts rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)
	outputAnnotations := annotations.New()

	response, rateLimit, err := b.client.GetWorkspace(ctx)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to get workspace: %w", err)
	}

	workspace := response.Data.Workspace

	// Create workspace resource with child resource type annotations
	resource, err := rs.NewResource(
		workspace.Name,
		workspaceResourceType,
		workspace.ID,
		rs.WithAnnotation(
			&v2.ChildResourceType{ResourceTypeId: userResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: groupResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: roleResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: inviteResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: sourceResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: warehouseResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: functionResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: spaceResourceType.Id},
		),
	)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to create workspace resource: %w", err)
	}

	l.Debug("listed workspace", zap.String("workspace_id", workspace.ID), zap.String("workspace_name", workspace.Name))

	return []*v2.Resource{resource}, &rs.SyncOpResults{Annotations: outputAnnotations}, nil
}

// Entitlements returns only the "member" entitlement for the workspace.
// Role-based entitlements live on role resources (role:{id}:member) to match baton-segment IDs.
func (b *workspaceBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	entitlements := []*v2.Entitlement{
		ent.NewAssignmentEntitlement(
			resource,
			workspaceMemberEntitlement,
			ent.WithDisplayName(fmt.Sprintf("%s Workspace Member", resource.DisplayName)),
			ent.WithDescription(fmt.Sprintf("Member of the %s Segment workspace", resource.DisplayName)),
			// Workspace membership is read-only: users are added via invites (inviteBuilder.CreateAccount),
			// not by granting this entitlement directly.
		),
	}
	return entitlements, nil, nil
}

// Grants returns all users as members of the workspace.
func (b *workspaceBuilder) Grants(ctx context.Context, resource *v2.Resource, opts rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)

	bag, err := parsePageToken(opts.PageToken.Token, &v2.ResourceId{ResourceType: userResourceType.Id})
	if err != nil {
		return nil, nil, err
	}

	pageToken := bag.PageToken()

	outputAnnotations := annotations.New()
	response, rateLimit, err := b.client.ListUsers(ctx, pageToken, client.DefaultPageSize)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to list workspace users: %w", err)
	}

	var grants []*v2.Grant
	for _, user := range response.Data.Users {
		userID := &v2.ResourceId{
			ResourceType: userResourceType.Id,
			Resource:     user.ID,
		}
		grants = append(grants, gr.NewGrant(resource, workspaceMemberEntitlement, userID))
	}

	nextToken, err := bag.NextToken(response.Data.Pagination.Next)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, err
	}

	l.Debug("listed workspace grants", zap.Int("count", len(grants)))

	return grants, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: outputAnnotations}, nil
}

// Grant handles the workspace member entitlement.
// Only users can be workspace members; membership is implicit when a user exists in the workspace.
func (b *workspaceBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) ([]*v2.Grant, annotations.Annotations, error) {
	if principal.Id.ResourceType != userResourceType.Id {
		return nil, nil, fmt.Errorf("only users can be granted workspace membership, got %s", principal.Id.ResourceType)
	}
	grant := gr.NewGrant(entitlement.Resource, workspaceMemberEntitlement, principal.Id)
	return []*v2.Grant{grant}, nil, nil
}

// Revoke is not supported for workspace membership; users must be deleted instead.
func (b *workspaceBuilder) Revoke(_ context.Context, _ *v2.Grant) (annotations.Annotations, error) {
	return nil, fmt.Errorf("workspace membership cannot be revoked directly; delete the user instead")
}

func newWorkspaceBuilder(c *client.Client) *workspaceBuilder {
	return &workspaceBuilder{client: c}
}
