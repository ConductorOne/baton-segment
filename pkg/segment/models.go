package segment

type User struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Email       string       `json:"email"`
	Permissions []Permission `json:"permissions"`
}

type Permission struct {
	RoleID    string     `json:"roleId"`
	RoleName  string     `json:"roleName"`
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
	Permissions []Permission `json:"permissions"`
}

type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
