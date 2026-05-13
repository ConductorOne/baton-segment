package main

import (
	"sync"
)

const (
	roleWorkspaceAdminID    = "role_workspace_admin"
	roleWorkspaceAdminName  = "Workspace Admin"
	roleSourceAdminID       = "role_source_admin"
	roleSourceAdminName     = "Source Admin"
	roleSourceReadonlyID    = "role_source_readonly"
	roleSourceReadonlyName  = "Source Read-only"
	roleDestAdminID         = "role_destination_admin"
	roleDestAdminName       = "Destination Admin"
	roleDestReadonlyID      = "role_destination_readonly"
	roleDestReadonlyName    = "Destination Read-only"
	roleWarehouseAdminID    = "role_warehouse_admin"
	roleWarehouseAdminName  = "Warehouse Admin"
	workspace001ID          = "workspace_001"
	user001ID               = "user_001"
	user002ID               = "user_002"
	user005ID               = "user_005"
	user006ID               = "user_006"
	source002ID             = "source_002"
	warehouse001ID          = "warehouse_001"
	source001ID             = "source_001"
	resourceTypeSource      = "SOURCE"
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
	ts.roles[roleWorkspaceAdminID] = Role{
		ID:          roleWorkspaceAdminID,
		Name:        roleWorkspaceAdminName,
		Description: "Administrative access to workspace settings and resources",
	}
	ts.roles[roleSourceAdminID] = Role{
		ID:          roleSourceAdminID,
		Name:        roleSourceAdminName,
		Description: "Full control over sources in the workspace",
	}
	ts.roles[roleSourceReadonlyID] = Role{
		ID:          roleSourceReadonlyID,
		Name:        roleSourceReadonlyName,
		Description: "Read-only access to sources",
	}
	ts.roles[roleDestAdminID] = Role{
		ID:          roleDestAdminID,
		Name:        roleDestAdminName,
		Description: "Full control over destinations in the workspace",
	}
	ts.roles[roleDestReadonlyID] = Role{
		ID:          roleDestReadonlyID,
		Name:        roleDestReadonlyName,
		Description: "Read-only access to destinations",
	}
	ts.roles[roleWarehouseAdminID] = Role{
		ID:          roleWarehouseAdminID,
		Name:        roleWarehouseAdminName,
		Description: "Full control over warehouses in the workspace",
	}
	ts.roles["role_function_admin"] = Role{
		ID:          "role_function_admin",
		Name:        "Function Admin",
		Description: "Full control over functions in the workspace",
	}

	// Workspace resource for permissions
	wsResource := Resource{ID: workspace001ID, Type: "WORKSPACE"}

	// Sample users with various roles (all permissions must have Resources)
	ts.users[user001ID] = User{
		ID:    user001ID,
		Name:  "Alice Johnson",
		Email: "alice.johnson@example.com",
		Permissions: []Permission{
			{RoleID: "role_workspace_owner", RoleName: "Workspace Owner", Resources: []Resource{wsResource}},
		},
	}
	ts.users[user002ID] = User{
		ID:    user002ID,
		Name:  "Bob Smith",
		Email: "bob.smith@example.com",
		Permissions: []Permission{
			{RoleID: roleWorkspaceAdminID, RoleName: roleWorkspaceAdminName, Resources: []Resource{wsResource}},
			{RoleID: roleSourceAdminID, RoleName: roleSourceAdminName, Resources: []Resource{{ID: source001ID, Type: resourceTypeSource}}},
		},
	}
	ts.users["user_003"] = User{
		ID:    "user_003",
		Name:  "Carol Williams",
		Email: "carol.williams@example.com",
		Permissions: []Permission{
			{RoleID: roleSourceReadonlyID, RoleName: roleSourceReadonlyName, Resources: []Resource{{ID: source002ID, Type: resourceTypeSource}}},
			{RoleID: roleDestReadonlyID, RoleName: roleDestReadonlyName, Resources: []Resource{wsResource}},
		},
	}
	ts.users["user_004"] = User{
		ID:    "user_004",
		Name:  "David Brown",
		Email: "david.brown@example.com",
		Permissions: []Permission{
			{RoleID: roleDestAdminID, RoleName: roleDestAdminName, Resources: []Resource{wsResource}},
		},
	}
	ts.users[user005ID] = User{
		ID:    user005ID,
		Name:  "Eve Davis",
		Email: "eve.davis@example.com",
		Permissions: []Permission{
			{RoleID: roleWarehouseAdminID, RoleName: roleWarehouseAdminName, Resources: []Resource{{ID: warehouse001ID, Type: "WAREHOUSE"}}},
			{RoleID: "role_function_admin", RoleName: "Function Admin", Resources: []Resource{{ID: "func_001", Type: "FUNCTION"}}},
		},
	}
	ts.users[user006ID] = User{
		ID:    user006ID,
		Name:  "Frank Miller",
		Email: "frank.miller@example.com",
		Permissions: []Permission{
			{RoleID: roleSourceReadonlyID, RoleName: roleSourceReadonlyName, Resources: []Resource{{ID: "source_003", Type: resourceTypeSource}}},
		},
	}
	ts.users["user_007"] = User{
		ID:    "user_007",
		Name:  "Grace Wilson",
		Email: "grace.wilson@example.com",
		Permissions: []Permission{
			{RoleID: roleDestReadonlyID, RoleName: roleDestReadonlyName, Resources: []Resource{wsResource}},
		},
	}

	// Sample groups with members and permissions
	ts.groups["group_001"] = Group{
		ID:          "group_001",
		Name:        "Engineering",
		MemberCount: 4,
		Members:     []string{user001ID, user002ID, user005ID, user006ID},
		Permissions: []Permission{
			{RoleID: roleSourceAdminID, RoleName: roleSourceAdminName, Resources: []Resource{{ID: source001ID, Type: resourceTypeSource}}},
			{RoleID: roleDestAdminID, RoleName: roleDestAdminName, Resources: []Resource{wsResource}},
		},
	}
	ts.groups["group_002"] = Group{
		ID:          "group_002",
		Name:        "Data Science",
		MemberCount: 3,
		Members:     []string{"user_003", "user_004", user005ID},
		Permissions: []Permission{
			{RoleID: roleWarehouseAdminID, RoleName: roleWarehouseAdminName, Resources: []Resource{{ID: warehouse001ID, Type: "WAREHOUSE"}}},
		},
	}
	ts.groups["group_003"] = Group{
		ID:          "group_003",
		Name:        "Marketing",
		MemberCount: 2,
		Members:     []string{user006ID, "user_007"},
		Permissions: []Permission{
			{RoleID: roleSourceReadonlyID, RoleName: roleSourceReadonlyName, Resources: []Resource{{ID: source002ID, Type: resourceTypeSource}}},
		},
	}
	ts.groups["group_004"] = Group{
		ID:          "group_004",
		Name:        "Administrators",
		MemberCount: 2,
		Members:     []string{user001ID, user002ID},
		Permissions: []Permission{
			{RoleID: roleWorkspaceAdminID, RoleName: roleWorkspaceAdminName, Resources: []Resource{wsResource}},
		},
	}

	// Sample pending invites
	ts.invites["pending.user1@example.com"] = true
	ts.invites["pending.user2@example.com"] = true
	ts.invites["newteam.member@example.com"] = true

	// Workspace (single workspace per token)
	ts.workspace = Workspace{
		ID:   workspace001ID,
		Name: "Test Workspace",
		Slug: "test-workspace",
	}

	// Sample sources
	ts.sources[source001ID] = Source{
		ID:          source001ID,
		Slug:        "javascript-source",
		Name:        "JavaScript Source",
		WorkspaceID: workspace001ID,
		Enabled:     true,
	}
	ts.sources[source002ID] = Source{
		ID:          source002ID,
		Slug:        "python-source",
		Name:        "Python Source",
		WorkspaceID: workspace001ID,
		Enabled:     true,
	}
	ts.sources["source_003"] = Source{
		ID:          "source_003",
		Slug:        "ios-source",
		Name:        "iOS Source",
		WorkspaceID: workspace001ID,
		Enabled:     false,
	}

	// Sample warehouses
	ts.warehouses[warehouse001ID] = Warehouse{
		ID:          warehouse001ID,
		WorkspaceID: workspace001ID,
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
		WorkspaceID: workspace001ID,
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
		WorkspaceID:  workspace001ID,
		DisplayName:  "Data Transform Function",
		Description:  "Transforms incoming data before sending to destinations",
		ResourceType: resourceTypeSource,
	}
	ts.functions["func_002"] = Function{
		ID:           "func_002",
		WorkspaceID:  workspace001ID,
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
