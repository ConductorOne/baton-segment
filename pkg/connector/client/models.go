package client

import "fmt"

// Pagination represents the pagination information in API responses.
type Pagination struct {
	Current   string `json:"current,omitempty"`
	Next      string `json:"next,omitempty"`
	Previous  string `json:"previous,omitempty"`
	TotalSize int    `json:"totalEntries,omitempty"`
}

// User represents a Segment IAM user.
type User struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Email       string       `json:"email"`
	Permissions []Permission `json:"permissions,omitempty"`
}

// Permission represents a permission assigned to a user or group.
type Permission struct {
	RoleID    string     `json:"roleId"`
	RoleName  string     `json:"roleName,omitempty"`
	Resources []Resource `json:"resources,omitempty"`
	Labels    []Label    `json:"labels,omitempty"`
}

// Resource represents a resource in a permission.
type Resource struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Label string `json:"label,omitempty"`
}

// Label represents a label in a permission.
type Label struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

// Group represents a Segment IAM user group.
type Group struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	MemberCount int          `json:"memberCount,omitempty"`
	Permissions []Permission `json:"permissions,omitempty"`
}

// Role represents a Segment IAM role.
type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Invite represents a pending workspace invitation.
type Invite struct {
	Email       string       `json:"email"`
	Permissions []Permission `json:"permissions,omitempty"`
}

// Workspace represents a Segment workspace.
type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// Source represents a Segment data source.
type Source struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	WorkspaceID string `json:"workspaceId"`
	Enabled     bool   `json:"enabled"`
}

// Warehouse represents a Segment data warehouse.
type Warehouse struct {
	ID          string             `json:"id"`
	WorkspaceID string             `json:"workspaceId"`
	Enabled     bool               `json:"enabled"`
	Metadata    *WarehouseMetadata `json:"metadata,omitempty"`
}

// WarehouseMetadata contains metadata about a warehouse.
type WarehouseMetadata struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Function represents a Segment function.
type Function struct {
	ID           string `json:"id"`
	WorkspaceID  string `json:"workspaceId"`
	DisplayName  string `json:"displayName"`
	Description  string `json:"description,omitempty"`
	ResourceType string `json:"resourceType"` // SOURCE or DESTINATION
}

// Space represents a Segment space (Personas environment).
type Space struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// PermissionInput represents a permission to be set on a user or group.
type PermissionInput struct {
	RoleID    string          `json:"roleId"`
	Resources []ResourceInput `json:"resources"`
}

// ResourceInput represents a resource for permission assignment.
type ResourceInput struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// ErrorResponse represents an error response from the Segment API.
type ErrorResponse struct {
	Errors []struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Field   string `json:"field,omitempty"`
	} `json:"errors"`
}

// Message implements the uhttp error response interface.
func (e *ErrorResponse) Message() string {
	if len(e.Errors) == 0 {
		return "unknown error"
	}
	return fmt.Sprintf("type: %s, message: %s", e.Errors[0].Type, e.Errors[0].Message)
}

// ListUsersResponse is the response from the list users endpoint.
type ListUsersResponse struct {
	Data struct {
		Users      []User     `json:"users"`
		Pagination Pagination `json:"pagination"`
	} `json:"data"`
}

// GetUserResponse is the response from the get user endpoint.
type GetUserResponse struct {
	Data struct {
		User User `json:"user"`
	} `json:"data"`
}

// ListGroupsResponse is the response from the list groups endpoint.
type ListGroupsResponse struct {
	Data struct {
		UserGroups []Group    `json:"userGroups"`
		Pagination Pagination `json:"pagination"`
	} `json:"data"`
}

// GetGroupResponse is the response from the get group endpoint.
type GetGroupResponse struct {
	Data struct {
		UserGroup Group `json:"userGroup"`
	} `json:"data"`
}

// ListGroupUsersResponse is the response from the list group users endpoint.
type ListGroupUsersResponse struct {
	Data struct {
		Users      []User     `json:"users"`
		Pagination Pagination `json:"pagination"`
	} `json:"data"`
}

// ListRolesResponse is the response from the list roles endpoint.
// Roles are system-defined by Segment (~8 built-in). No pagination needed.
type ListRolesResponse struct {
	Data struct {
		Roles []Role `json:"roles"`
	} `json:"data"`
}

// ListInvitesResponse is the response from the list invites endpoint.
type ListInvitesResponse struct {
	Data struct {
		Invites    []string   `json:"invites"` // Invites are just email strings
		Pagination Pagination `json:"pagination"`
	} `json:"data"`
}

// CreateInvitesRequest is the request body for creating invites.
type CreateInvitesRequest struct {
	Invites []InviteRequest `json:"invites"`
}

// InviteRequest represents a single invite request.
type InviteRequest struct {
	Email       string            `json:"email"`
	Permissions []PermissionInput `json:"permissions,omitempty"`
}

// CreateInvitesResponse is the response from the create invites endpoint.
type CreateInvitesResponse struct {
	Data struct {
		Emails []string `json:"emails"`
	} `json:"data"`
}

// AddUsersToGroupRequest is the request body for adding users to a group.
type AddUsersToGroupRequest struct {
	Emails []string `json:"emails"`
}

// AddUsersToGroupResponse is the response from adding users to a group.
type AddUsersToGroupResponse struct {
	Data struct {
		UserGroup Group `json:"userGroup"`
	} `json:"data"`
}

// GetWorkspaceResponse is the response from the get workspace endpoint.
type GetWorkspaceResponse struct {
	Data struct {
		Workspace Workspace `json:"workspace"`
	} `json:"data"`
}

// ListSourcesResponse is the response from the list sources endpoint.
type ListSourcesResponse struct {
	Data struct {
		Sources    []Source   `json:"sources"`
		Pagination Pagination `json:"pagination"`
	} `json:"data"`
}

// ListWarehousesResponse is the response from the list warehouses endpoint.
type ListWarehousesResponse struct {
	Data struct {
		Warehouses []Warehouse `json:"warehouses"`
		Pagination Pagination  `json:"pagination"`
	} `json:"data"`
}

// ListFunctionsResponse is the response from the list functions endpoint.
type ListFunctionsResponse struct {
	Data struct {
		Functions  []Function `json:"functions"`
		Pagination Pagination `json:"pagination"`
	} `json:"data"`
}

// ListSpacesResponse is the response from the list spaces endpoint.
type ListSpacesResponse struct {
	Data struct {
		Spaces     []Space    `json:"spaces"`
		Pagination Pagination `json:"pagination"`
	} `json:"data"`
}

// UpdatePermissionsRequest is the request body for updating permissions.
type UpdatePermissionsRequest struct {
	Permissions []PermissionInput `json:"permissions"`
}

// UpdatePermissionsResponse is the response from updating permissions.
type UpdatePermissionsResponse struct {
	Data struct {
		Permissions []Permission `json:"permissions"`
	} `json:"data"`
}
