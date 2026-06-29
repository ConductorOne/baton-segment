package connector

import (
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
)

// Resource type constants for permission scoping.
const (
	ResourceTypeWorkspace = "WORKSPACE"
	ResourceTypeSource    = "SOURCE"
	ResourceTypeWarehouse = "WAREHOUSE"
	ResourceTypeFunction  = "FUNCTION"
	ResourceTypeSpace     = "SPACE"
)

// Resource type for the workspace (top-level container).
var workspaceResourceType = &v2.ResourceType{
	Id:          "workspace",
	DisplayName: "Workspace",
	Description: "A Segment workspace",
}

// Resource type for IAM users.
// Users return grants (role assignments) but no entitlements.
var userResourceType = &v2.ResourceType{
	Id:          "user",
	DisplayName: "User",
	Description: "A Segment workspace user",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_USER},
	Annotations: annotations.New(&v2.SkipEntitlements{}),
}

// Resource type for IAM groups.
var groupResourceType = &v2.ResourceType{
	Id:          "group",
	DisplayName: "Group",
	Description: "A Segment user group",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_GROUP},
}

// Resource type for IAM roles.
var roleResourceType = &v2.ResourceType{
	Id:          "role",
	DisplayName: "Role",
	Description: "A Segment IAM role",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_ROLE},
}

// Resource type for pending invitations.
// Invites don't have entitlements or grants - they're just pending users.
var inviteResourceType = &v2.ResourceType{
	Id:          "invite",
	DisplayName: "Invite",
	Description: "A pending workspace invitation",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_USER},
	Annotations: annotations.New(
		&v2.SkipEntitlementsAndGrants{},
		&v2.SkipSyncAnomalyDetection{},
	),
}

// Resource type for data sources.
// Sources have role-based entitlements (e.g., Source Admin, Source Read-only).
var sourceResourceType = &v2.ResourceType{
	Id:          "source",
	DisplayName: "Source",
	Description: "A Segment data source",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_APP},
	Annotations: annotations.New(&v2.SkipGrants{}),
}

// Resource type for data warehouses.
// Warehouses have role-based entitlements but grants are synced from users/groups.
var warehouseResourceType = &v2.ResourceType{
	Id:          "warehouse",
	DisplayName: "Warehouse",
	Description: "A Segment data warehouse",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_APP},
	Annotations: annotations.New(&v2.SkipGrants{}),
}

// Resource type for functions.
// Functions have role-based entitlements but grants are synced from users/groups.
var functionResourceType = &v2.ResourceType{
	Id:          "function",
	DisplayName: "Function",
	Description: "A Segment function",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_APP},
	Annotations: annotations.New(&v2.SkipGrants{}),
}

// Resource type for spaces.
// Spaces have role-based entitlements but grants are synced from users/groups.
var spaceResourceType = &v2.ResourceType{
	Id:          "space",
	DisplayName: "Space",
	Description: "A Segment space (Personas environment)",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_APP},
	Annotations: annotations.New(&v2.SkipGrants{}),
}
