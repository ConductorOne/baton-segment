package segment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const (
	BaseUrl = "https://api.segmentapis.com/"

	groups      = "groups"
	users       = "users"
	roles       = "roles"
	sources     = "sources"
	warehouses  = "warehouses"
	functions   = "functions"
	spaces      = "spaces"
	permissions = "permissions"
)

type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type Pagination struct {
	Current      string `json:"current"`
	Next         string `json:"next,omitempty"`
	TotalEntries int64  `json:"totalEntries"`
}

type Payload struct {
	Emails []string `json:"emails"`
}

type Client struct {
	httpClient *http.Client
	token      string
}

type PermissionsPayload struct {
	Permissions []Permission `json:"permissions"`
}

func NewClient(httpClient *http.Client, token string) *Client {
	return &Client{
		httpClient: httpClient,
		token:      token,
	}
}

// ListUsers returns a list of all users.
func (c *Client) ListUsers(ctx context.Context, cursor string) ([]User, string, error) {
	var res struct {
		Data struct {
			Users      []User     `json:"users"`
			Pagination Pagination `json:"pagination"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	params := c.setParams(cursor)
	url, _ := url.JoinPath(BaseUrl, users)
	if err := c.doRequest(ctx, url, &res, http.MethodGet, params, nil); err != nil {
		return nil, "", err
	}

	if res.Data.Pagination.Next != "" {
		return res.Data.Users, res.Data.Pagination.Next, nil
	}

	if res.Errors != nil {
		return nil, "", fmt.Errorf("error fetching users: %s - %s", res.Errors[0].Type, res.Errors[0].Message)
	}

	return res.Data.Users, "", nil
}

// ListSources returns a list of all sources.
func (c *Client) ListSources(ctx context.Context, cursor string) ([]Source, string, error) {
	var res struct {
		Data struct {
			Sources    []Source   `json:"sources"`
			Pagination Pagination `json:"pagination"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	params := c.setParams(cursor)
	url, _ := url.JoinPath(BaseUrl, sources)
	if err := c.doRequest(ctx, url, &res, http.MethodGet, params, nil); err != nil {
		return nil, "", err
	}

	if res.Data.Pagination.Next != "" {
		return res.Data.Sources, res.Data.Pagination.Next, nil
	}

	if res.Errors != nil {
		return nil, "", fmt.Errorf("error fetching sources: %s - %s", res.Errors[0].Type, res.Errors[0].Message)
	}

	return res.Data.Sources, "", nil
}

// ListWarehouses returns a list of all warehouses.
func (c *Client) ListWarehouses(ctx context.Context, cursor string) ([]Warehouse, string, error) {
	var res struct {
		Data struct {
			Warehouses []Warehouse `json:"warehouses"`
			Pagination Pagination  `json:"pagination"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	params := c.setParams(cursor)
	url, _ := url.JoinPath(BaseUrl, sources)
	if err := c.doRequest(ctx, url, &res, http.MethodGet, params, nil); err != nil {
		return nil, "", err
	}

	if res.Data.Pagination.Next != "" {
		return res.Data.Warehouses, res.Data.Pagination.Next, nil
	}

	if res.Errors != nil {
		return nil, "", fmt.Errorf("error fetching warehouses: %s - %s", res.Errors[0].Type, res.Errors[0].Message)
	}

	return res.Data.Warehouses, "", nil
}

// ListFunctions returns a list of all functions.
func (c *Client) ListFunctions(ctx context.Context, cursor string, fnType string) ([]Function, string, error) {
	var res struct {
		Data struct {
			Functions  []Function `json:"functions"`
			Pagination Pagination `json:"pagination"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	params := c.setParams(cursor)
	params.Add("resourceType", fnType)
	url, _ := url.JoinPath(BaseUrl, functions)
	if err := c.doRequest(ctx, url, &res, http.MethodGet, params, nil); err != nil {
		return nil, "", err
	}

	if res.Data.Pagination.Next != "" {
		return res.Data.Functions, res.Data.Pagination.Next, nil
	}

	if res.Errors != nil {
		return nil, "", fmt.Errorf("error fetching functions: %s - %s", res.Errors[0].Type, res.Errors[0].Message)
	}

	return res.Data.Functions, "", nil
}

// ListSpaces returns a list of all spaces.
func (c *Client) ListSpaces(ctx context.Context, cursor string) ([]Space, string, error) {
	var res struct {
		Data struct {
			Spaces     []Space    `json:"spaces"`
			Pagination Pagination `json:"pagination"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	params := c.setParams(cursor)
	url, _ := url.JoinPath(BaseUrl, sources)
	if err := c.doRequest(ctx, url, &res, http.MethodGet, params, nil); err != nil {
		return nil, "", err
	}

	if res.Data.Pagination.Next != "" {
		return res.Data.Spaces, res.Data.Pagination.Next, nil
	}

	if res.Errors != nil {
		return nil, "", fmt.Errorf("error fetching functions: %s - %s", res.Errors[0].Type, res.Errors[0].Message)
	}

	return res.Data.Spaces, "", nil
}

// ListGroups returns a list of all user groups.
func (c *Client) ListGroups(ctx context.Context, cursor string) ([]Group, string, error) {
	var res struct {
		Data struct {
			Groups     []Group    `json:"userGroups"`
			Pagination Pagination `json:"pagination"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	params := c.setParams(cursor)
	url, _ := url.JoinPath(BaseUrl, groups)
	if err := c.doRequest(ctx, url, &res, http.MethodGet, params, nil); err != nil {
		return nil, "", err
	}

	if res.Data.Pagination.Next != "" {
		return res.Data.Groups, res.Data.Pagination.Next, nil
	}

	if res.Errors != nil {
		return nil, "", fmt.Errorf("error fetching groups: %s - %s", res.Errors[0].Type, res.Errors[0].Message)
	}

	return res.Data.Groups, "", nil
}

// ListGroupMembers returns a list of all user group members.
func (c *Client) ListGroupMembers(ctx context.Context, groupId, cursor string) ([]User, string, error) {
	var res struct {
		Data struct {
			Users      []User     `json:"users"`
			Pagination Pagination `json:"pagination"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	params := c.setParams(cursor)
	url, _ := url.JoinPath(BaseUrl, groups, groupId, users)
	if err := c.doRequest(ctx, url, &res, http.MethodGet, params, nil); err != nil {
		return nil, "", err
	}

	if res.Data.Pagination.Next != "" {
		return res.Data.Users, res.Data.Pagination.Next, nil
	}

	if res.Errors != nil {
		return nil, "", fmt.Errorf("error fetching group members: %s - %s", res.Errors[0].Type, res.Errors[0].Message)
	}

	return res.Data.Users, "", nil
}

// GetWorkspace returns workspace of a current token.
func (c *Client) GetWorkspace(ctx context.Context) (*Workspace, error) {
	var res struct {
		Data struct {
			Workspace Workspace `json:"workspace"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	if err := c.doRequest(ctx, BaseUrl, &res, http.MethodGet, nil, nil); err != nil {
		return nil, err
	}

	if res.Errors != nil {
		return nil, fmt.Errorf("error fetching a workspace: %s - %s", res.Errors[0].Type, res.Errors[0].Message)
	}

	return &res.Data.Workspace, nil
}

// GetUser returns single user details.
func (c *Client) GetUser(ctx context.Context, userID string) (*User, error) {
	var res struct {
		Data struct {
			User User `json:"user"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	url, _ := url.JoinPath(BaseUrl, users, userID)
	if err := c.doRequest(ctx, url, &res, http.MethodGet, nil, nil); err != nil {
		return nil, err
	}

	if res.Errors != nil {
		return nil, fmt.Errorf("error fetching user: %s - %s", res.Errors[0].Type, res.Errors[0].Message)
	}

	return &res.Data.User, nil
}

// GetGroup returns single group details.
func (c *Client) GetGroup(ctx context.Context, groupID string) (*Group, error) {
	var res struct {
		Data struct {
			Group Group `json:"group"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	url, _ := url.JoinPath(BaseUrl, groups, groupID)
	if err := c.doRequest(ctx, url, &res, http.MethodGet, nil, nil); err != nil {
		return nil, err
	}

	if res.Errors != nil {
		return nil, fmt.Errorf("error fetching group: %s - %s", res.Errors[0].Type, res.Errors[0].Message)
	}

	return &res.Data.Group, nil
}

// ListRoles returns a list of all roles.
func (c *Client) ListRoles(ctx context.Context, cursor string) ([]Role, string, error) {
	var res struct {
		Data struct {
			Roles      []Role     `json:"roles"`
			Pagination Pagination `json:"pagination"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	params := c.setParams(cursor)
	url, _ := url.JoinPath(BaseUrl, roles)
	if err := c.doRequest(ctx, url, &res, http.MethodGet, params, nil); err != nil {
		return nil, "", err
	}

	if res.Data.Pagination.Next != "" {
		return res.Data.Roles, res.Data.Pagination.Next, nil
	}

	if res.Errors != nil {
		return nil, "", fmt.Errorf("error fetching roles: %s - %s", res.Errors[0].Type, res.Errors[0].Message)
	}

	return res.Data.Roles, "", nil
}

// AddGroupMembers adds user to a group.
func (c *Client) AddGroupMembers(ctx context.Context, groupId, userEmail string) error {
	url, _ := url.JoinPath(BaseUrl, groups, groupId, users)
	body := Payload{
		Emails: []string{userEmail},
	}

	var res struct {
		Data struct {
			UserGroup Group `json:"userGroup"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	e := c.doRequest(ctx, url, &res, http.MethodPost, nil, body)
	if e != nil {
		return e
	}

	if res.Errors != nil {
		return fmt.Errorf("error adding user to a group: %s - %s", res.Errors[0].Type, res.Errors[0].Message)
	}

	return nil
}

// UpdatePermissions updates permissions for a user or a group.
func (c *Client) UpdatePermissions(ctx context.Context, principalId, principalType string, newPermissions []Permission) error {
	var principal string
	if principalType == "user" {
		principal = users
	} else {
		principal = groups
	}

	url, _ := url.JoinPath(BaseUrl, principal, principalId, permissions)
	body := PermissionsPayload{Permissions: newPermissions}
	var res struct {
		Data struct {
			Permission []Permission `json:"permissions"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	e := c.doRequest(ctx, url, &res, http.MethodPut, nil, body)
	if e != nil {
		return e
	}

	if res.Errors != nil {
		return fmt.Errorf("error adding permissions to %s: %s - %s", principalType, res.Errors[0].Type, res.Errors[0].Message)
	}

	return nil
}

// RemoveGroupMember removes member from the group.
func (c *Client) RemoveGroupMember(ctx context.Context, groupId, userEmail string) error {
	url, _ := url.JoinPath(BaseUrl, groups, groupId, users)
	var res struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	params := c.setParams("")
	emailParamValue, _ := json.Marshal([]string{userEmail})
	params.Add("emails", string(emailParamValue))
	err := c.doRequest(ctx, url, res, http.MethodDelete, params, nil)
	if err != nil {
		return fmt.Errorf("failed to remove user from group: %w", err)
	}

	if res.Data.Status != "SUCCESS" {
		return fmt.Errorf("failed to remove user from group")
	}

	return nil
}

func (c *Client) doRequest(ctx context.Context, path string, res interface{}, method string, params url.Values, payload interface{}) error {
	var body []byte
	var err error

	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, path, bytes.NewReader(body))
	if err != nil {
		return err
	}

	if params != nil {
		req.URL.RawQuery = params.Encode()
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/vnd.segment.v1+json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.token))
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}

	return nil
}

func (c *Client) setParams(cursor string) url.Values {
	query := url.Values{}
	query.Add("pagination[count]", fmt.Sprint(200))
	if cursor != "" {
		query.Add("pagination[cursor]", cursor)
	}

	return query
}
