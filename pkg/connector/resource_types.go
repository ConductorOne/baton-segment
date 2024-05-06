package connector

import (
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

var (
	userResourceType = &v2.ResourceType{
		Id:          "user",
		DisplayName: "User",
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_USER},
	}
	workspaceResourceType = &v2.ResourceType{
		Id:          "workspace",
		DisplayName: "Workspace",
	}
	groupResourceType = &v2.ResourceType{
		Id:          "group",
		DisplayName: "Group",
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_GROUP},
	}
	roleResourceType = &v2.ResourceType{
		Id:          "role",
		DisplayName: "Role",
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_ROLE},
	}
	sourceResourceType = &v2.ResourceType{
		Id:          "source",
		DisplayName: "Source",
	}
	warehouseResourceType = &v2.ResourceType{
		Id:          "warehouse",
		DisplayName: "Warehouse",
	}
	functionResourceType = &v2.ResourceType{
		Id:          "function",
		DisplayName: "Function",
	}
	spaceResourceType = &v2.ResourceType{
		Id:          "space",
		DisplayName: "Space",
	}
)
