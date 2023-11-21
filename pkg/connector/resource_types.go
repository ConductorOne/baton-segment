package connector

import (
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

var (
	userResourceType = &v2.ResourceType{
		Id:          "user",
		DisplayName: "User",
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_USER},
		Annotations: annotationsForUserResourceType(),
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
	resourceResourceType = &v2.ResourceType{
		Id:          "resource",
		DisplayName: "Resource",
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_ROLE},
	}
)
