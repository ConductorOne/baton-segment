package main

import (
	"sync"
)

// User represents a Segment IAM user.
type User struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Email       string       `json:"email"`
	Permissions []Permission `json:"permissions,omitempty"`
}

// Group represents a Segment IAM user group.
type Group struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	MemberCount int          `json:"memberCount"`
	Members     []string     `json:"members,omitempty"` // User IDs
	Permissions []Permission `json:"permissions,omitempty"`
}

// Role represents a Segment IAM role.
type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Permission represents a permission assigned to a user or group.
type Permission struct {
	RoleID    string     `json:"roleId"`
	RoleName  string     `json:"roleName,omitempty"`
	Resources []Resource `json:"resources,omitempty"`
}

// Resource represents a resource in a permission.
type Resource struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Label string `json:"label,omitempty"`
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

// TestServer holds the mock data and state.
type TestServer struct {
	workspace  Workspace
	users      map[string]User
	groups     map[string]Group
	roles      map[string]Role
	invites    map[string]bool // email -> pending status
	sources    map[string]Source
	warehouses map[string]Warehouse
	functions  map[string]Function
	spaces     map[string]Space
	mu         sync.RWMutex
}

// NewTestServer creates a new test server with sample data.
func NewTestServer() *TestServer {
	ts := &TestServer{
		users:      make(map[string]User),
		groups:     make(map[string]Group),
		roles:      make(map[string]Role),
		invites:    make(map[string]bool),
		sources:    make(map[string]Source),
		warehouses: make(map[string]Warehouse),
		functions:  make(map[string]Function),
		spaces:     make(map[string]Space),
	}
	ts.initSampleData()
	return ts
}

// initSampleData populates the test server with sample data.
func (ts *TestServer) initSampleData() {
	// Sample roles
	ts.roles["role_workspace_owner"] = Role{
		ID:          "role_workspace_owner",
		Name:        "Workspace Owner",
		Description: "Full control over the workspace including billing and team management",
	}
	ts.roles["role_workspace_admin"] = Role{
		ID:          "role_workspace_admin",
		Name:        "Workspace Admin",
		Description: "Administrative access to workspace settings and resources",
	}
	ts.roles["role_source_admin"] = Role{
		ID:          "role_source_admin",
		Name:        "Source Admin",
		Description: "Full control over sources in the workspace",
	}
	ts.roles["role_source_readonly"] = Role{
		ID:          "role_source_readonly",
		Name:        "Source Read-only",
		Description: "Read-only access to sources",
	}
	ts.roles["role_destination_admin"] = Role{
		ID:          "role_destination_admin",
		Name:        "Destination Admin",
		Description: "Full control over destinations in the workspace",
	}
	ts.roles["role_destination_readonly"] = Role{
		ID:          "role_destination_readonly",
		Name:        "Destination Read-only",
		Description: "Read-only access to destinations",
	}
	ts.roles["role_warehouse_admin"] = Role{
		ID:          "role_warehouse_admin",
		Name:        "Warehouse Admin",
		Description: "Full control over warehouses in the workspace",
	}
	ts.roles["role_function_admin"] = Role{
		ID:          "role_function_admin",
		Name:        "Function Admin",
		Description: "Full control over functions in the workspace",
	}

	// Workspace resource for permissions
	wsResource := Resource{ID: "workspace_001", Type: "WORKSPACE"}

	// Sample users with various roles (all permissions must have Resources)
	ts.users["user_001"] = User{
		ID:    "user_001",
		Name:  "Alice Johnson",
		Email: "alice.johnson@example.com",
		Permissions: []Permission{
			{RoleID: "role_workspace_owner", RoleName: "Workspace Owner", Resources: []Resource{wsResource}},
		},
	}
	ts.users["user_002"] = User{
		ID:    "user_002",
		Name:  "Bob Smith",
		Email: "bob.smith@example.com",
		Permissions: []Permission{
			{RoleID: "role_workspace_admin", RoleName: "Workspace Admin", Resources: []Resource{wsResource}},
			{RoleID: "role_source_admin", RoleName: "Source Admin", Resources: []Resource{{ID: "source_001", Type: "SOURCE"}}},
		},
	}
	ts.users["user_003"] = User{
		ID:    "user_003",
		Name:  "Carol Williams",
		Email: "carol.williams@example.com",
		Permissions: []Permission{
			{RoleID: "role_source_readonly", RoleName: "Source Read-only", Resources: []Resource{{ID: "source_002", Type: "SOURCE"}}},
			{RoleID: "role_destination_readonly", RoleName: "Destination Read-only", Resources: []Resource{wsResource}},
		},
	}
	ts.users["user_004"] = User{
		ID:    "user_004",
		Name:  "David Brown",
		Email: "david.brown@example.com",
		Permissions: []Permission{
			{RoleID: "role_destination_admin", RoleName: "Destination Admin", Resources: []Resource{wsResource}},
		},
	}
	ts.users["user_005"] = User{
		ID:    "user_005",
		Name:  "Eve Davis",
		Email: "eve.davis@example.com",
		Permissions: []Permission{
			{RoleID: "role_warehouse_admin", RoleName: "Warehouse Admin", Resources: []Resource{{ID: "warehouse_001", Type: "WAREHOUSE"}}},
			{RoleID: "role_function_admin", RoleName: "Function Admin", Resources: []Resource{{ID: "func_001", Type: "FUNCTION"}}},
		},
	}
	ts.users["user_006"] = User{
		ID:    "user_006",
		Name:  "Frank Miller",
		Email: "frank.miller@example.com",
		Permissions: []Permission{
			{RoleID: "role_source_readonly", RoleName: "Source Read-only", Resources: []Resource{{ID: "source_003", Type: "SOURCE"}}},
		},
	}
	ts.users["user_007"] = User{
		ID:    "user_007",
		Name:  "Grace Wilson",
		Email: "grace.wilson@example.com",
		Permissions: []Permission{
			{RoleID: "role_destination_readonly", RoleName: "Destination Read-only", Resources: []Resource{wsResource}},
		},
	}

	// Sample groups with members and permissions
	ts.groups["group_001"] = Group{
		ID:          "group_001",
		Name:        "Engineering",
		MemberCount: 4,
		Members:     []string{"user_001", "user_002", "user_005", "user_006"},
		Permissions: []Permission{
			{RoleID: "role_source_admin", RoleName: "Source Admin", Resources: []Resource{{ID: "source_001", Type: "SOURCE"}}},
			{RoleID: "role_destination_admin", RoleName: "Destination Admin", Resources: []Resource{wsResource}},
		},
	}
	ts.groups["group_002"] = Group{
		ID:          "group_002",
		Name:        "Data Science",
		MemberCount: 3,
		Members:     []string{"user_003", "user_004", "user_005"},
		Permissions: []Permission{
			{RoleID: "role_warehouse_admin", RoleName: "Warehouse Admin", Resources: []Resource{{ID: "warehouse_001", Type: "WAREHOUSE"}}},
		},
	}
	ts.groups["group_003"] = Group{
		ID:          "group_003",
		Name:        "Marketing",
		MemberCount: 2,
		Members:     []string{"user_006", "user_007"},
		Permissions: []Permission{
			{RoleID: "role_source_readonly", RoleName: "Source Read-only", Resources: []Resource{{ID: "source_002", Type: "SOURCE"}}},
		},
	}
	ts.groups["group_004"] = Group{
		ID:          "group_004",
		Name:        "Administrators",
		MemberCount: 2,
		Members:     []string{"user_001", "user_002"},
		Permissions: []Permission{
			{RoleID: "role_workspace_admin", RoleName: "Workspace Admin", Resources: []Resource{wsResource}},
		},
	}

	// Sample pending invites
	ts.invites["pending.user1@example.com"] = true
	ts.invites["pending.user2@example.com"] = true
	ts.invites["newteam.member@example.com"] = true

	// Workspace (single workspace per token)
	ts.workspace = Workspace{
		ID:   "workspace_001",
		Name: "Test Workspace",
		Slug: "test-workspace",
	}

	// Sample sources
	ts.sources["source_001"] = Source{
		ID:          "source_001",
		Slug:        "javascript-source",
		Name:        "JavaScript Source",
		WorkspaceID: "workspace_001",
		Enabled:     true,
	}
	ts.sources["source_002"] = Source{
		ID:          "source_002",
		Slug:        "python-source",
		Name:        "Python Source",
		WorkspaceID: "workspace_001",
		Enabled:     true,
	}
	ts.sources["source_003"] = Source{
		ID:          "source_003",
		Slug:        "ios-source",
		Name:        "iOS Source",
		WorkspaceID: "workspace_001",
		Enabled:     false,
	}

	// Sample warehouses
	ts.warehouses["warehouse_001"] = Warehouse{
		ID:          "warehouse_001",
		WorkspaceID: "workspace_001",
		Enabled:     true,
		Metadata: &WarehouseMetadata{
			ID:          "bigquery",
			Slug:        "bigquery",
			Name:        "BigQuery",
			Description: "Google BigQuery data warehouse",
		},
	}
	ts.warehouses["warehouse_002"] = Warehouse{
		ID:          "warehouse_002",
		WorkspaceID: "workspace_001",
		Enabled:     true,
		Metadata: &WarehouseMetadata{
			ID:          "snowflake",
			Slug:        "snowflake",
			Name:        "Snowflake",
			Description: "Snowflake data warehouse",
		},
	}

	// Sample functions
	ts.functions["func_001"] = Function{
		ID:           "func_001",
		WorkspaceID:  "workspace_001",
		DisplayName:  "Data Transform Function",
		Description:  "Transforms incoming data before sending to destinations",
		ResourceType: "SOURCE",
	}
	ts.functions["func_002"] = Function{
		ID:           "func_002",
		WorkspaceID:  "workspace_001",
		DisplayName:  "Custom Destination Function",
		Description:  "Custom destination for internal analytics",
		ResourceType: "DESTINATION",
	}

	// Sample spaces
	ts.spaces["space_001"] = Space{
		ID:   "space_001",
		Name: "Production Space",
		Slug: "production-space",
	}
	ts.spaces["space_002"] = Space{
		ID:   "space_002",
		Name: "Development Space",
		Slug: "development-space",
	}
}

// getUserByEmailLocked finds a user by email without acquiring locks.
// Caller must hold at least a read lock.
func (ts *TestServer) getUserByEmailLocked(email string) *User {
	for _, user := range ts.users {
		if user.Email == email {
			return &user
		}
	}
	return nil
}

// updateGroupMemberCount updates the member count for a group.
func (ts *TestServer) updateGroupMemberCount(groupID string) {
	if group, exists := ts.groups[groupID]; exists {
		group.MemberCount = len(group.Members)
		ts.groups[groupID] = group
	}
}
