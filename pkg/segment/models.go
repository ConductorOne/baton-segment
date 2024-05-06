package segment

type User struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Email       string       `json:"email"`
	Permissions []Permission `json:"permissions,omitempty"`
}

type Permission struct {
	RoleID    string     `json:"roleId"`
	RoleName  string     `json:"roleName,omitempty"`
	Resources []Resource `json:"resources"`
}

type Resource struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Group struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	MemberCount int64        `json:"memberCount"`
	Permissions []Permission `json:"permissions,omitempty"`
}

type Role struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Resources   []Resource `json:"resources,omitempty"`
}

type Source struct {
	ID          string        `json:"id"`
	Slug        string        `json:"slug"`
	Name        string        `json:"name"`
	WorkspaceID string        `json:"workspaceId"`
	Enabled     bool          `json:"enabled"`
	WriteKeys   []string      `json:"writeKeys"`
	Metadata    Metadata      `json:"metadata"`
	Labels      []interface{} `json:"labels"`
}

type Metadata struct {
	ID                 string   `json:"id"`
	Slug               string   `json:"slug"`
	Name               string   `json:"name"`
	Categories         []string `json:"categories"`
	Description        string   `json:"description"`
	Options            []Option `json:"options"`
	IsCloudEventSource bool     `json:"isCloudEventSource"`
	Logos              Logos    `json:"logos"`
}

type Logos struct {
	Default string `json:"default"`
	Alt     string `json:"alt"`
}

type Warehouse struct {
	ID          string   `json:"id"`
	WorkspaceID string   `json:"workspaceId"`
	Enabled     bool     `json:"enabled"`
	Metadata    Metadata `json:"metadata"`
}

type Option struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Type     string `json:"type"`
}

type Function struct {
	ID                string    `json:"id"`
	WorkspaceID       string    `json:"workspaceId"`
	DisplayName       string    `json:"displayName"`
	Description       string    `json:"description"`
	LogoURL           string    `json:"logoUrl"`
	Code              string    `json:"code"`
	CreatedAt         string    `json:"createdAt"`
	CreatedBy         string    `json:"createdBy"`
	PreviewWebhookURL string    `json:"previewWebhookUrl"`
	Settings          []Setting `json:"settings"`
	Buildpack         string    `json:"buildpack"`
	CatalogID         string    `json:"catalogId"`
	BatchMaxCount     int64     `json:"batchMaxCount"`
	ResourceType      string    `json:"resourceType"`
}

type Setting struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Sensitive   bool   `json:"sensitive"`
}

type Space struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type PermissionRes struct {
	PolicyID    string     `json:"policyId"`
	RoleName    string     `json:"roleName"`
	RoleID      string     `json:"roleId"`
	SubjectID   string     `json:"subjectId"`
	SubjectType string     `json:"subjectType"`
	Resources   []Resource `json:"resources"`
}
