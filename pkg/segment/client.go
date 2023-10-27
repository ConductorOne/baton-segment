package segment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const BaseUrl = "https://api.segmentapis.com/"

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

func NewClient(httpClient *http.Client, token string) *Client {
	return &Client{
		httpClient: httpClient,
		token:      token,
	}
}

const (
	groups = "groups"
	users  = "users"
	roles  = "roles"
)

// ListUsers returns a list of all users.
func (c *Client) ListUsers(ctx context.Context, cursor string) ([]User, string, error) {
	var res struct {
		Data struct {
			Users      []User     `json:"users"`
			Pagination Pagination `json:"pagination"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	url, _ := url.JoinPath(BaseUrl, users)
	if err := c.doRequest(ctx, url, &res, cursor, nil, http.MethodGet, ""); err != nil {
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

// ListGroups returns a list of all user groups.
func (c *Client) ListGroups(ctx context.Context, cursor string) ([]Group, string, error) {
	var res struct {
		Data struct {
			Groups     []Group    `json:"userGroups"`
			Pagination Pagination `json:"pagination"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	url, _ := url.JoinPath(BaseUrl, groups)
	if err := c.doRequest(ctx, url, &res, cursor, nil, http.MethodGet, ""); err != nil {
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

	url, _ := url.JoinPath(BaseUrl, groups, groupId, users)
	if err := c.doRequest(ctx, url, &res, cursor, nil, http.MethodGet, ""); err != nil {
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

	url, _ := url.JoinPath(BaseUrl, "")
	if err := c.doRequest(ctx, url, &res, "", nil, http.MethodGet, ""); err != nil {
		return nil, err
	}

	if res.Errors != nil {
		return nil, fmt.Errorf("error fetching a workspace: %s - %s", res.Errors[0].Type, res.Errors[0].Message)
	}

	return &res.Data.Workspace, nil
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

	url, _ := url.JoinPath(BaseUrl, roles)
	if err := c.doRequest(ctx, url, &res, cursor, nil, http.MethodGet, ""); err != nil {
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
	body := []Payload{
		{
			Emails: []string{userEmail},
		},
	}

	requestBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	var res struct {
		Data struct {
			UserGroup Group `json:"userGroup"`
		} `json:"data,omitempty"`
		Errors []Error `json:"errors,omitempty"`
	}

	e := c.doRequest(ctx, url, &res, "", requestBody, http.MethodPost, "")
	if e != nil {
		return e
	}

	if res.Errors != nil {
		return fmt.Errorf("error adding user to a group: %s - %s", res.Errors[0].Type, res.Errors[0].Message)
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

	err := c.doRequest(ctx, url, res, "", nil, http.MethodDelete, userEmail)
	if err != nil {
		return fmt.Errorf("failed to remove user from group: %w", err)
	}

	if res.Data.Status != "SUCCESS" {
		return fmt.Errorf("failed to remove user from group")
	}

	return nil
}

// TODO: refactor this method to work with query params in a better way.
func (c *Client) doRequest(ctx context.Context, path string, res interface{}, cursor string, payload []byte, method string, userEmail string) error {
	req, err := http.NewRequestWithContext(ctx, method, path, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	query := req.URL.Query()
	query.Add("pagination[count]", fmt.Sprint(100))
	if cursor != "" {
		query.Add("pagination[cursor]", cursor)
	}

	// TODO: remove this from here and find a better way to pass query params.
	if userEmail != "" {
		emailParamValue, _ := json.Marshal([]string{userEmail})
		query.Add("emails", string(emailParamValue))
	}

	req.URL.RawQuery = query.Encode()
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
