package connector

import (
	"context"
	"fmt"
	"strings"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-segment/pkg/segment"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/iancoleman/strcase"
	"go.uber.org/zap"
)

func parsePageToken(i string, resourceID *v2.ResourceId) (*pagination.Bag, string, error) {
	b := &pagination.Bag{}
	err := b.Unmarshal(i)
	if err != nil {
		return nil, "", err
	}

	if b.Current() == nil {
		b.Push(pagination.PageState{
			ResourceTypeID: resourceID.ResourceType,
			ResourceID:     resourceID.Resource,
		})
	}

	return b, b.PageToken(), nil
}

func grantPermissions(ctx context.Context, client *segment.Client, principal *v2.Resource, roleID, resourceType, resourceID string) ([]segment.Permission, error) {
	l := ctxzap.Extract(ctx)
	var permissions []segment.Permission

	switch principal.Id.ResourceType {
	case userResourceType.Id:
		user, err := client.GetUser(ctx, principal.Id.Resource)
		if err != nil {
			return nil, fmt.Errorf("baton-segment: failed to get user info while granting permission: %w", err)
		}
		permissions = user.Permissions
	case groupResourceType.Id:
		group, err := client.GetGroup(ctx, principal.Id.Resource)
		if err != nil {
			return nil, fmt.Errorf("baton-segment: failed to get group info while granting permission: %w", err)
		}
		permissions = group.Permissions
	default:
		l.Warn(
			"baton-segment: only users and groups can be granted permissions",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("baton-segment: only users and groups can be granted permissions")
	}

	permissions = append(permissions, segment.Permission{
		RoleID: roleID,
		Resources: []segment.Resource{
			{ID: resourceID, Type: resourceType},
		},
	})

	return permissions, nil
}

func revokePermissions(ctx context.Context, client *segment.Client, principal *v2.Resource, roleID string) ([]segment.Permission, error) {
	var newPermissions []segment.Permission
	l := ctxzap.Extract(ctx)

	switch principal.Id.ResourceType {
	case userResourceType.Id:
		user, err := client.GetUser(ctx, principal.Id.Resource)
		if err != nil {
			return nil, fmt.Errorf("baton-segment: failed to get user info while revoking role: %w", err)
		}
		for _, permission := range user.Permissions {
			if permission.RoleID != roleID {
				newPermissions = append(newPermissions, permission)
			}
		}
	case groupResourceType.Id:
		group, err := client.GetGroup(ctx, principal.Id.Resource)
		if err != nil {
			return nil, fmt.Errorf("baton-segment: failed to get group info while revoking role: %w", err)
		}
		for _, permission := range group.Permissions {
			if permission.RoleID != roleID {
				newPermissions = append(newPermissions, permission)
			}
		}
	default:
		l.Warn(
			"baton-segment: only users and groups can have permissions revoked",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("baton-segment: only users and groups can have permissions revoked")
	}

	return newPermissions, nil
}

func createEntitlement(role segment.Role, resource *v2.Resource) *v2.Entitlement {
	permissionOptions := []ent.EntitlementOption{
		ent.WithGrantableTo(userResourceType, groupResourceType),
		ent.WithDisplayName(fmt.Sprintf("%s resource %s", resource.DisplayName, role.Name)),
		// need to store role ID to access it later when granting permission.
		ent.WithDescription(fmt.Sprintf("%s:%s:%s", role.Name, role.ID, role.Description)),
	}

	roleToEntitlement := strcase.ToSnake(role.Name)
	entitlement := ent.NewPermissionEntitlement(
		resource,
		roleToEntitlement,
		permissionOptions...,
	)
	return entitlement
}

func getRoleIdAndResourceType(entitlement *v2.Entitlement) (string, string) {
	entitlementDesc := strings.Split(entitlement.Description, ":")
	roleID := entitlementDesc[1]
	resourceType := strings.ToUpper(entitlement.Resource.Id.ResourceType)

	return roleID, resourceType
}

// baseResource used to create resource associated with a role.
func baseResource(resource segment.Resource, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	ret, err := rs.NewResource(
		resource.Type,
		functionResourceType,
		resource.ID,
		rs.WithParentResourceID(parentResourceID),
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
