package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

const (
	// DefaultPageSize is the default number of items per page.
	DefaultPageSize = 1000

	// API Endpoint Paths
	// Documentation: https://docs.segmentapis.com/

	// usersPath is the API path for listing users.
	// API Docs: https://docs.segmentapis.com/tag/IAM-Users#operation/listUsers
	usersPath = "/users"

	// userByIDPath is the API path template for user operations by ID.
	// API Docs: https://docs.segmentapis.com/tag/IAM-Users#operation/getUser
	userByIDPath = "/users/%s"

	// groupsPath is the API path for listing user groups.
	// API Docs: https://docs.segmentapis.com/tag/IAM-Groups#operation/listUserGroups
	groupsPath = "/groups"

	// groupByIDPath is the API path template for group operations by ID.
	// API Docs: https://docs.segmentapis.com/tag/IAM-Groups#operation/getUserGroup
	groupByIDPath = "/groups/%s"

	// groupUsersPath is the API path template for group user operations.
	// API Docs: https://docs.segmentapis.com/tag/IAM-Groups#operation/listUserGroupsFromUser
	groupUsersPath = "/groups/%s/users"

	// rolesPath is the API path for listing roles.
	// API Docs: https://docs.segmentapis.com/tag/IAM-Roles#operation/listRoles
	rolesPath = "/roles"

	// invitesPath is the API path for invite operations.
	// API Docs: https://docs.segmentapis.com/tag/IAM-Invites#operation/listInvites
	invitesPath = "/invites"

	// workspacePath is the API path for getting the current workspace.
	// API Docs: https://docs.segmentapis.com/tag/Workspace#operation/getWorkspace
	workspacePath = "/"

	// sourcesPath is the API path for listing sources.
	// API Docs: https://docs.segmentapis.com/tag/Sources#operation/listSources
	sourcesPath = "/sources"

	// warehousesPath is the API path for listing warehouses.
	// API Docs: https://docs.segmentapis.com/tag/Warehouses#operation/listWarehouses
	warehousesPath = "/warehouses"

	// functionsPath is the API path for listing functions.
	// API Docs: https://docs.segmentapis.com/tag/Functions#operation/listFunctions
	functionsPath = "/functions"

	// spacesPath is the API path for listing spaces.
	// API Docs: https://docs.segmentapis.com/tag/Spaces#operation/listSpaces
	spacesPath = "/spaces"

	// userPermissionsPath is the API path template for user permissions.
	// API Docs: https://docs.segmentapis.com/tag/IAM-Users#operation/addPermissionsToUser
	// API Docs: https://docs.segmentapis.com/tag/IAM-Users#operation/replacePermissionsForUser
	userPermissionsPath = "/users/%s/permissions"
)

// Client is the HTTP client for the Segment API.
type Client struct {
	httpClient  *uhttp.BaseHttpClient
	baseURL     string
	accessToken string
}

// New creates a new Segment API client.
func New(ctx context.Context, accessToken, baseURL string) (*Client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid base URL %q: must include scheme and host", baseURL)
	}

	httpClient, err := uhttp.NewClient(ctx, uhttp.WithLogger(true, ctxzap.Extract(ctx)))
	if err != nil {
		return nil, fmt.Errorf("failed to create http client: %w", err)
	}

	baseClient, err := uhttp.NewBaseHttpClientWithContext(ctx, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create base http client: %w", err)
	}

	return &Client{
		httpClient:  baseClient,
		baseURL:     parsed.String(),
		accessToken: accessToken,
	}, nil
}

// doRequest performs an HTTP request to the Segment API.
// It handles rate limiting and error responses.
// Returns the rate limit description and any error that occurred.
func (c *Client) doRequest(
	ctx context.Context,
	method, path string,
	query url.Values,
	body interface{},
	response interface{},
) (*v2.RateLimitDescription, error) {
	rawURL, err := url.JoinPath(c.baseURL, path)
	if err != nil {
		return nil, fmt.Errorf("failed to build url: %w", err)
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}

	if query != nil {
		parsedURL.RawQuery = query.Encode()
	}

	opts := []uhttp.RequestOption{
		uhttp.WithBearerToken(c.accessToken),
		uhttp.WithAcceptJSONHeader(),
		uhttp.WithContentTypeJSONHeader(),
	}

	if body != nil {
		opts = append(opts, uhttp.WithJSONBody(body))
	}

	req, err := c.httpClient.NewRequest(ctx, method, parsedURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var ratelimitData v2.RateLimitDescription
	doOptions := []uhttp.DoOption{
		uhttp.WithRatelimitData(&ratelimitData),
		uhttp.WithErrorResponse(&ErrorResponse{}),
	}

	if response != nil {
		doOptions = append(doOptions, uhttp.WithJSONResponse(response))
	}

	resp, err := c.httpClient.Do(req, doOptions...)
	if err != nil {
		return &ratelimitData, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	return &ratelimitData, nil
}

// ListUsers returns all users in the workspace.
func (c *Client) ListUsers(ctx context.Context, cursor string, pageSize int) (*ListUsersResponse, *v2.RateLimitDescription, error) {
	query := url.Values{}
	query.Set("pagination.count", strconv.Itoa(pageSize))
	if cursor != "" {
		query.Set("pagination.cursor", cursor)
	}

	var response ListUsersResponse
	rl, err := c.doRequest(ctx, http.MethodGet, usersPath, query, nil, &response)
	if err != nil {
		return nil, rl, fmt.Errorf("failed to list users: %w", err)
	}

	return &response, rl, nil
}

// GetUser returns a single user by ID.
func (c *Client) GetUser(ctx context.Context, userID string) (*GetUserResponse, *v2.RateLimitDescription, error) {
	var response GetUserResponse
	path := fmt.Sprintf(userByIDPath, userID)
	rl, err := c.doRequest(ctx, http.MethodGet, path, nil, nil, &response)
	if err != nil {
		return nil, rl, fmt.Errorf("failed to get user %s: %w", userID, err)
	}

	return &response, rl, nil
}

// DeleteUser removes a user from the workspace.
func (c *Client) DeleteUser(ctx context.Context, userID string) (*v2.RateLimitDescription, error) {
	query := url.Values{}
	query.Set("userIds.0", userID)

	rl, err := c.doRequest(ctx, http.MethodDelete, usersPath, query, nil, nil)
	if err != nil {
		return rl, fmt.Errorf("failed to delete user %s: %w", userID, err)
	}

	return rl, nil
}

// ListGroups returns all groups in the workspace.
func (c *Client) ListGroups(ctx context.Context, cursor string, pageSize int) (*ListGroupsResponse, *v2.RateLimitDescription, error) {
	query := url.Values{}
	query.Set("pagination.count", strconv.Itoa(pageSize))
	if cursor != "" {
		query.Set("pagination.cursor", cursor)
	}

	var response ListGroupsResponse
	rl, err := c.doRequest(ctx, http.MethodGet, groupsPath, query, nil, &response)
	if err != nil {
		return nil, rl, fmt.Errorf("failed to list groups: %w", err)
	}

	return &response, rl, nil
}

// GetGroup returns a single group by ID with its permissions.
func (c *Client) GetGroup(ctx context.Context, groupID string) (*GetGroupResponse, *v2.RateLimitDescription, error) {
	var response GetGroupResponse
	path := fmt.Sprintf(groupByIDPath, groupID)
	rl, err := c.doRequest(ctx, http.MethodGet, path, nil, nil, &response)
	if err != nil {
		return nil, rl, fmt.Errorf("failed to get group %s: %w", groupID, err)
	}

	return &response, rl, nil
}

// ListGroupUsers returns all users in a group.
func (c *Client) ListGroupUsers(ctx context.Context, groupID string, cursor string, pageSize int) (*ListGroupUsersResponse, *v2.RateLimitDescription, error) {
	query := url.Values{}
	query.Set("pagination.count", strconv.Itoa(pageSize))
	if cursor != "" {
		query.Set("pagination.cursor", cursor)
	}

	var response ListGroupUsersResponse
	path := fmt.Sprintf(groupUsersPath, groupID)
	rl, err := c.doRequest(ctx, http.MethodGet, path, query, nil, &response)
	if err != nil {
		return nil, rl, fmt.Errorf("failed to list group users for group %s: %w", groupID, err)
	}

	return &response, rl, nil
}

// AddUsersToGroup adds users to a group by email.
func (c *Client) AddUsersToGroup(ctx context.Context, groupID string, emails []string) (*AddUsersToGroupResponse, *v2.RateLimitDescription, error) {
	body := AddUsersToGroupRequest{Emails: emails}

	var response AddUsersToGroupResponse
	path := fmt.Sprintf(groupUsersPath, groupID)
	rl, err := c.doRequest(ctx, http.MethodPost, path, nil, body, &response)
	if err != nil {
		return nil, rl, fmt.Errorf("failed to add users to group %s: %w", groupID, err)
	}

	return &response, rl, nil
}

// RemoveUsersFromGroup removes users from a group by email.
func (c *Client) RemoveUsersFromGroup(ctx context.Context, groupID string, emails []string) (*v2.RateLimitDescription, error) {
	if len(emails) == 0 {
		return nil, nil
	}
	query := url.Values{}
	for i, email := range emails {
		query.Set(fmt.Sprintf("emails.%d", i), email)
	}

	path := fmt.Sprintf(groupUsersPath, groupID)
	rl, err := c.doRequest(ctx, http.MethodDelete, path, query, nil, nil)
	if err != nil {
		return rl, fmt.Errorf("failed to remove users from group %s: %w", groupID, err)
	}

	return rl, nil
}

// ListRoles returns all roles in the workspace.
// Roles are system-defined by Segment (~8 built-in), so no pagination is needed.
func (c *Client) ListRoles(ctx context.Context) (*ListRolesResponse, *v2.RateLimitDescription, error) {
	var response ListRolesResponse
	rl, err := c.doRequest(ctx, http.MethodGet, rolesPath, nil, nil, &response)
	if err != nil {
		return nil, rl, fmt.Errorf("failed to list roles: %w", err)
	}

	return &response, rl, nil
}

// ListInvites returns all pending invites in the workspace.
func (c *Client) ListInvites(ctx context.Context, cursor string, pageSize int) (*ListInvitesResponse, *v2.RateLimitDescription, error) {
	query := url.Values{}
	query.Set("pagination.count", strconv.Itoa(pageSize))
	if cursor != "" {
		query.Set("pagination.cursor", cursor)
	}

	var response ListInvitesResponse
	rl, err := c.doRequest(ctx, http.MethodGet, invitesPath, query, nil, &response)
	if err != nil {
		return nil, rl, fmt.Errorf("failed to list invites: %w", err)
	}

	return &response, rl, nil
}

// FindPendingInviteByEmail paginates every page of pending invites looking
// for a case-insensitive email match. Segment exposes no email filter on
// this endpoint. Returns ("", false, nil, nil) if the full list is exhausted
// with no match. The returned rate limit description is the last non-nil one
// observed across all pages scanned.
func (c *Client) FindPendingInviteByEmail(ctx context.Context, email string) (string, bool, *v2.RateLimitDescription, error) {
	cursor := ""
	var lastRateLimit *v2.RateLimitDescription
	for {
		response, rl, err := c.ListInvites(ctx, cursor, DefaultPageSize)
		if rl != nil {
			lastRateLimit = rl
		}
		if err != nil {
			return "", false, lastRateLimit, fmt.Errorf("find pending invite by email: %w", err)
		}
		for _, inviteEmail := range response.Data.Invites {
			if strings.EqualFold(inviteEmail, email) {
				return inviteEmail, true, lastRateLimit, nil
			}
		}
		cursor = response.Data.Pagination.Next
		if cursor == "" {
			return "", false, lastRateLimit, nil
		}
	}
}

// CreateInvites creates new workspace invitations.
func (c *Client) CreateInvites(ctx context.Context, invites []InviteRequest) (*CreateInvitesResponse, *v2.RateLimitDescription, error) {
	body := CreateInvitesRequest{Invites: invites}

	var response CreateInvitesResponse
	rl, err := c.doRequest(ctx, http.MethodPost, invitesPath, nil, body, &response)
	if err != nil {
		return nil, rl, fmt.Errorf("failed to create invites: %w", err)
	}

	return &response, rl, nil
}

// DeleteInvites removes pending invitations by email.
func (c *Client) DeleteInvites(ctx context.Context, emails []string) (*v2.RateLimitDescription, error) {
	if len(emails) == 0 {
		return nil, nil
	}
	query := url.Values{}
	for i, email := range emails {
		query.Set(fmt.Sprintf("emails.%d", i), email)
	}

	rl, err := c.doRequest(ctx, http.MethodDelete, invitesPath, query, nil, nil)
	if err != nil {
		return rl, fmt.Errorf("failed to delete invites: %w", err)
	}

	return rl, nil
}

// ValidateCredentials makes a simple API call to verify the access token is valid.
func (c *Client) ValidateCredentials(ctx context.Context) (*v2.RateLimitDescription, error) {
	_, rl, err := c.ListRoles(ctx)
	if err != nil {
		return rl, fmt.Errorf("failed to validate credentials: %w", err)
	}
	return rl, nil
}

// GetWorkspace returns the workspace associated with the access token.
func (c *Client) GetWorkspace(ctx context.Context) (*GetWorkspaceResponse, *v2.RateLimitDescription, error) {
	var response GetWorkspaceResponse
	rl, err := c.doRequest(ctx, http.MethodGet, workspacePath, nil, nil, &response)
	if err != nil {
		return nil, rl, fmt.Errorf("failed to get workspace: %w", err)
	}

	return &response, rl, nil
}

// ListSources returns all sources in the workspace.
func (c *Client) ListSources(ctx context.Context, cursor string, pageSize int) (*ListSourcesResponse, *v2.RateLimitDescription, error) {
	query := url.Values{}
	query.Set("pagination.count", strconv.Itoa(pageSize))
	if cursor != "" {
		query.Set("pagination.cursor", cursor)
	}

	var response ListSourcesResponse
	rl, err := c.doRequest(ctx, http.MethodGet, sourcesPath, query, nil, &response)
	if err != nil {
		return nil, rl, fmt.Errorf("failed to list sources: %w", err)
	}

	return &response, rl, nil
}

// ListWarehouses returns all warehouses in the workspace.
func (c *Client) ListWarehouses(ctx context.Context, cursor string, pageSize int) (*ListWarehousesResponse, *v2.RateLimitDescription, error) {
	query := url.Values{}
	query.Set("pagination.count", strconv.Itoa(pageSize))
	if cursor != "" {
		query.Set("pagination.cursor", cursor)
	}

	var response ListWarehousesResponse
	rl, err := c.doRequest(ctx, http.MethodGet, warehousesPath, query, nil, &response)
	if err != nil {
		return nil, rl, fmt.Errorf("failed to list warehouses: %w", err)
	}

	return &response, rl, nil
}

// ListFunctions returns all functions in the workspace.
func (c *Client) ListFunctions(ctx context.Context, cursor string, pageSize int, resourceType string) (*ListFunctionsResponse, *v2.RateLimitDescription, error) {
	query := url.Values{}
	query.Set("pagination.count", strconv.Itoa(pageSize))
	if cursor != "" {
		query.Set("pagination.cursor", cursor)
	}
	if resourceType != "" {
		query.Set("resourceType", resourceType)
	}

	var response ListFunctionsResponse
	rl, err := c.doRequest(ctx, http.MethodGet, functionsPath, query, nil, &response)
	if err != nil {
		return nil, rl, fmt.Errorf("failed to list functions: %w", err)
	}

	return &response, rl, nil
}

// ListSpaces returns all spaces in the workspace.
func (c *Client) ListSpaces(ctx context.Context, cursor string, pageSize int) (*ListSpacesResponse, *v2.RateLimitDescription, error) {
	query := url.Values{}
	query.Set("pagination.count", strconv.Itoa(pageSize))
	if cursor != "" {
		query.Set("pagination.cursor", cursor)
	}

	var response ListSpacesResponse
	rl, err := c.doRequest(ctx, http.MethodGet, spacesPath, query, nil, &response)
	if err != nil {
		return nil, rl, fmt.Errorf("failed to list spaces: %w", err)
	}

	return &response, rl, nil
}

// AddUserPermissions adds permissions to a user (appends to existing).
// API Docs: https://docs.segmentapis.com/tag/IAM-Users#operation/addPermissionsToUser
func (c *Client) AddUserPermissions(ctx context.Context, userID string, permissions []PermissionInput) (*UpdatePermissionsResponse, *v2.RateLimitDescription, error) {
	body := UpdatePermissionsRequest{Permissions: permissions}

	var response UpdatePermissionsResponse
	path := fmt.Sprintf(userPermissionsPath, userID)
	rl, err := c.doRequest(ctx, http.MethodPost, path, nil, body, &response)
	if err != nil {
		return nil, rl, fmt.Errorf("failed to add user permissions: %w", err)
	}

	return &response, rl, nil
}

// ReplaceUserPermissions replaces all permissions for a user (overwrites existing).
// API Docs: https://docs.segmentapis.com/tag/IAM-Users#operation/replacePermissionsForUser
func (c *Client) ReplaceUserPermissions(ctx context.Context, userID string, permissions []PermissionInput) (*UpdatePermissionsResponse, *v2.RateLimitDescription, error) {
	body := UpdatePermissionsRequest{Permissions: permissions}

	var response UpdatePermissionsResponse
	path := fmt.Sprintf(userPermissionsPath, userID)
	rl, err := c.doRequest(ctx, http.MethodPut, path, nil, body, &response)
	if err != nil {
		return nil, rl, fmt.Errorf("failed to replace user permissions: %w", err)
	}

	return &response, rl, nil
}
