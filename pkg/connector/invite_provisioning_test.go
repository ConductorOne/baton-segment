package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	gr "github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-segment/pkg/connector/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	testWorkspaceID = "workspace_001"
	testRoleID      = "role_workspace_admin"
	testInviteEmail = "new.user@example.com"
)

// apiCall records a request the connector made against the fake Segment API.
type apiCall struct {
	Method string
	Path   string
	Query  url.Values
	Body   string
}

// route is "METHOD /path", e.g. "POST /invites".
func (c apiCall) route() string {
	return fmt.Sprintf("%s %s", c.Method, c.Path)
}

// fakeSegment is a minimal stand-in for the Segment public API. Routes are keyed by
// "METHOD /path"; an unrouted request fails the test instead of silently 404ing, so a
// connector change that calls an unexpected endpoint is caught.
type fakeSegment struct {
	t      *testing.T
	server *httptest.Server

	mu     sync.Mutex
	calls  []apiCall
	routes map[string][]func(w http.ResponseWriter)
}

func newFakeSegment(t *testing.T) *fakeSegment {
	t.Helper()

	f := &fakeSegment{t: t, routes: map[string][]func(w http.ResponseWriter){}}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)

	return f
}

// respond registers responses for a route. Each call to the route consumes the next
// registered response; the last one is reused once they run out.
func (f *fakeSegment) respond(route string, responses ...func(w http.ResponseWriter)) {
	f.routes[route] = append(f.routes[route], responses...)
}

// respondJSON registers a single JSON response with the given status code.
func (f *fakeSegment) respondJSON(route string, statusCode int, body string) {
	f.respond(route, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	})
}

func (f *fakeSegment) serve(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	require.NoError(f.t, err)

	call := apiCall{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Body: string(body)}

	f.mu.Lock()
	f.calls = append(f.calls, call)
	responses := f.routes[call.route()]
	if len(responses) == 0 {
		f.mu.Unlock()
		f.t.Errorf("unexpected request to %s", call.route())
		w.WriteHeader(http.StatusNotFound)
		return
	}
	respond := responses[0]
	if len(responses) > 1 {
		f.routes[call.route()] = responses[1:]
	}
	f.mu.Unlock()

	respond(w)
}

// routes returns every request the connector made, in order.
func (f *fakeSegment) routesCalled() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	called := make([]string, 0, len(f.calls))
	for _, c := range f.calls {
		called = append(called, c.route())
	}
	return called
}

// bodiesFor returns the request bodies sent to a route, in order.
func (f *fakeSegment) bodiesFor(route string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	var bodies []string
	for _, c := range f.calls {
		if c.route() == route {
			bodies = append(bodies, c.Body)
		}
	}
	return bodies
}

// queriesFor returns the query strings sent to a route, in order.
func (f *fakeSegment) queriesFor(route string) []url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()

	var queries []url.Values
	for _, c := range f.calls {
		if c.route() == route {
			queries = append(queries, c.Query)
		}
	}
	return queries
}

func (f *fakeSegment) client() *client.Client {
	f.t.Helper()

	c, err := client.New(context.Background(), "test-token", f.server.URL)
	require.NoError(f.t, err)
	return c
}

func testWorkspaceResourceID() *v2.ResourceId {
	return &v2.ResourceId{ResourceType: workspaceResourceType.Id, Resource: testWorkspaceID}
}

// testInvitePrincipal builds the resource the SDK hands to Grant for somebody who was
// invited but has no Segment account yet.
func testInvitePrincipal(t *testing.T, email string) *v2.Resource {
	t.Helper()

	principal, err := inviteResource(email, testWorkspaceResourceID())
	require.NoError(t, err)
	return principal
}

func testUserPrincipal(t *testing.T, userID, email string) *v2.Resource {
	t.Helper()

	principal, err := userResource(&client.User{ID: userID, Name: "Existing User", Email: email}, testWorkspaceResourceID())
	require.NoError(t, err)
	return principal
}

// testRoleEntitlement builds the workspace-scoped role:{id}:member entitlement.
func testRoleEntitlement(t *testing.T) *v2.Entitlement {
	t.Helper()

	roleRes, err := roleResource(&client.Role{ID: testRoleID, Name: "Workspace Admin"}, testWorkspaceResourceID())
	require.NoError(t, err)
	return ent.NewAssignmentEntitlement(roleRes, roleMembership)
}

func decodeInviteRequest(t *testing.T, body string) client.CreateInvitesRequest {
	t.Helper()

	var req client.CreateInvitesRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))
	return req
}

// TestRoleGrantToInviteAssignsRoleAtInviteTime covers the reported bug: granting a role
// to somebody without a Segment account used to fail with "only users can be granted role
// membership, got invite". The role now travels with the invitation instead.
func TestRoleGrantToInviteAssignsRoleAtInviteTime(t *testing.T) {
	ctx := context.Background()

	fake := newFakeSegment(t)
	fake.respondJSON("POST /invites", http.StatusCreated, `{"data":{"emails":["`+testInviteEmail+`"]}}`)

	b := newRoleBuilder(fake.client())
	grants, _, err := b.Grant(ctx, testInvitePrincipal(t, testInviteEmail), testRoleEntitlement(t))
	require.NoError(t, err)

	// One POST /invites, and nothing sent to the user permissions endpoint, which is what
	// rejected invite principals.
	assert.Equal(t, []string{"POST /invites"}, fake.routesCalled())

	bodies := fake.bodiesFor("POST /invites")
	require.Len(t, bodies, 1)
	req := decodeInviteRequest(t, bodies[0])
	require.Len(t, req.Invites, 1)
	assert.Equal(t, testInviteEmail, req.Invites[0].Email)
	require.Len(t, req.Invites[0].Permissions, 1)
	assert.Equal(t, testRoleID, req.Invites[0].Permissions[0].RoleID)
	assert.Equal(t,
		[]client.ResourceInput{{ID: testWorkspaceID, Type: ResourceTypeWorkspace}},
		req.Invites[0].Permissions[0].Resources,
	)

	require.Len(t, grants, 1)
	assert.Equal(t, inviteResourceType.Id, grants[0].Principal.Id.ResourceType)
	assert.Equal(t, testInviteEmail, grants[0].Principal.Id.Resource)
}

// TestRoleGrantToInviteReissuesPendingInvitation covers the ConductorOne ordering:
// the account is created first (POST /invites without permissions) and the role granted
// afterwards. Segment refuses a second invitation for the same email and cannot patch a
// pending one, so the invitation is withdrawn and re-issued carrying the role.
func TestRoleGrantToInviteReissuesPendingInvitation(t *testing.T) {
	ctx := context.Background()

	fake := newFakeSegment(t)
	fake.respondJSON("POST /invites", http.StatusConflict,
		`{"errors":[{"type":"Conflict","message":"user has already been invited to this workspace"}]}`)
	fake.respondJSON("POST /invites", http.StatusCreated, `{"data":{"emails":["`+testInviteEmail+`"]}}`)
	fake.respondJSON("DELETE /invites", http.StatusOK, `{"data":{"status":"SUCCESS"}}`)

	b := newRoleBuilder(fake.client())
	grants, _, err := b.Grant(ctx, testInvitePrincipal(t, testInviteEmail), testRoleEntitlement(t))
	require.NoError(t, err)
	require.Len(t, grants, 1)

	assert.Equal(t, []string{"POST /invites", "DELETE /invites", "POST /invites"}, fake.routesCalled())

	queries := fake.queriesFor("DELETE /invites")
	require.Len(t, queries, 1)
	assert.Equal(t, testInviteEmail, queries[0].Get("emails.0"))

	// Both attempts carry the permissions: the retry must not fall back to a bare invite.
	for _, body := range fake.bodiesFor("POST /invites") {
		req := decodeInviteRequest(t, body)
		require.Len(t, req.Invites, 1)
		require.Len(t, req.Invites[0].Permissions, 1)
		assert.Equal(t, testRoleID, req.Invites[0].Permissions[0].RoleID)
	}
}

// TestRoleGrantToInviteSurfacesOtherErrors makes sure the withdraw-and-re-issue path is
// only taken for a duplicate invitation: an unrelated failure must not delete the invite.
func TestRoleGrantToInviteSurfacesOtherErrors(t *testing.T) {
	ctx := context.Background()

	fake := newFakeSegment(t)
	fake.respondJSON("POST /invites", http.StatusUnprocessableEntity,
		`{"errors":[{"type":"ValidationFailure","message":"roleId is not valid"}]}`)

	b := newRoleBuilder(fake.client())
	_, _, err := b.Grant(ctx, testInvitePrincipal(t, testInviteEmail), testRoleEntitlement(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to invite")
	assert.Equal(t, []string{"POST /invites"}, fake.routesCalled())
}

// TestRoleGrantToUserUnchanged pins the existing behaviour for provisioned users.
func TestRoleGrantToUserUnchanged(t *testing.T) {
	ctx := context.Background()

	fake := newFakeSegment(t)
	fake.respondJSON("POST /users/user_001/permissions", http.StatusOK, `{"data":{"permissions":[]}}`)

	b := newRoleBuilder(fake.client())
	grants, _, err := b.Grant(ctx, testUserPrincipal(t, "user_001", "existing@example.com"), testRoleEntitlement(t))
	require.NoError(t, err)
	require.Len(t, grants, 1)

	assert.Equal(t, []string{"POST /users/user_001/permissions"}, fake.routesCalled())
}

// TestRoleGrantRejectsUnsupportedPrincipal keeps group principals (and anything else)
// out of the role grant path.
func TestRoleGrantRejectsUnsupportedPrincipal(t *testing.T) {
	ctx := context.Background()

	fake := newFakeSegment(t)

	principal, err := rs.NewGroupResource("Engineering", groupResourceType, "group_001", nil)
	require.NoError(t, err)

	b := newRoleBuilder(fake.client())
	_, _, err = b.Grant(ctx, principal, testRoleEntitlement(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only users and invites can be granted role membership")
	assert.Empty(t, fake.routesCalled())
}

// TestRoleRevokeFromPendingInviteWithdrawsInvitation: Segment cannot modify a pending
// invitation's permissions, so the only way to stop the role from being applied is to
// withdraw the invitation.
func TestRoleRevokeFromPendingInviteWithdrawsInvitation(t *testing.T) {
	ctx := context.Background()

	fake := newFakeSegment(t)
	fake.respondJSON("GET /invites", http.StatusOK,
		`{"data":{"invites":["someone.else@example.com","`+testInviteEmail+`"],"pagination":{"current":"MA=="}}}`)
	fake.respondJSON("DELETE /invites", http.StatusOK, `{"data":{"status":"SUCCESS"}}`)

	b := newRoleBuilder(fake.client())
	principal := testInvitePrincipal(t, testInviteEmail)
	grant := gr.NewGrant(testRoleEntitlement(t).Resource, roleMembership, principal)

	annos, err := b.Revoke(ctx, grant)
	require.NoError(t, err)
	assert.False(t, annos.Contains(&v2.GrantAlreadyRevoked{}))

	assert.Equal(t, []string{"GET /invites", "DELETE /invites"}, fake.routesCalled())
	queries := fake.queriesFor("DELETE /invites")
	require.Len(t, queries, 1)
	assert.Equal(t, testInviteEmail, queries[0].Get("emails.0"))
}

// TestRoleRevokeFromAcceptedInviteRevokesFromUser: once the invitation is accepted the
// permission lives on a real user, so revoking must reach that user instead of reporting
// success while the access is still in place.
func TestRoleRevokeFromAcceptedInviteRevokesFromUser(t *testing.T) {
	ctx := context.Background()

	fake := newFakeSegment(t)
	fake.respondJSON("GET /invites", http.StatusOK,
		`{"data":{"invites":["someone.else@example.com"],"pagination":{"current":"MA=="}}}`)
	fake.respondJSON("GET /users", http.StatusOK,
		`{"data":{"users":[{"id":"user_010","name":"New User","email":"`+testInviteEmail+`"}],"pagination":{"current":"MA=="}}}`)
	fake.respondJSON("GET /users/user_010", http.StatusOK, `{"data":{"user":{
		"id":"user_010",
		"email":"`+testInviteEmail+`",
		"permissions":[
			{"roleId":"`+testRoleID+`","resources":[{"id":"`+testWorkspaceID+`","type":"WORKSPACE"}]},
			{"roleId":"role_source_admin","resources":[{"id":"source_001","type":"SOURCE"}]}
		]}}}`)
	fake.respondJSON("PUT /users/user_010/permissions", http.StatusOK, `{"data":{"permissions":[]}}`)

	b := newRoleBuilder(fake.client())
	principal := testInvitePrincipal(t, testInviteEmail)
	grant := gr.NewGrant(testRoleEntitlement(t).Resource, roleMembership, principal)

	annos, err := b.Revoke(ctx, grant)
	require.NoError(t, err)
	assert.False(t, annos.Contains(&v2.GrantAlreadyRevoked{}))

	assert.Equal(t,
		[]string{"GET /invites", "GET /users", "GET /users/user_010", "PUT /users/user_010/permissions"},
		fake.routesCalled(),
	)

	bodies := fake.bodiesFor("PUT /users/user_010/permissions")
	require.Len(t, bodies, 1)

	var replace client.UpdatePermissionsRequest
	require.NoError(t, json.Unmarshal([]byte(bodies[0]), &replace))
	assert.Equal(t, []client.PermissionInput{{
		RoleID:    "role_source_admin",
		Resources: []client.ResourceInput{{ID: "source_001", Type: ResourceTypeSource}},
	}}, replace.Permissions)
}

// TestRoleRevokeFromInviteWithNoAccount reports an already-revoked grant when neither a
// pending invitation nor a user exists for the email.
func TestRoleRevokeFromInviteWithNoAccount(t *testing.T) {
	ctx := context.Background()

	fake := newFakeSegment(t)
	fake.respondJSON("GET /invites", http.StatusOK, `{"data":{"invites":[],"pagination":{"current":"MA=="}}}`)
	fake.respondJSON("GET /users", http.StatusOK, `{"data":{"users":[],"pagination":{"current":"MA=="}}}`)

	b := newRoleBuilder(fake.client())
	principal := testInvitePrincipal(t, testInviteEmail)
	grant := gr.NewGrant(testRoleEntitlement(t).Resource, roleMembership, principal)

	annos, err := b.Revoke(ctx, grant)
	require.NoError(t, err)
	assert.True(t, annos.Contains(&v2.GrantAlreadyRevoked{}))
	assert.Equal(t, []string{"GET /invites", "GET /users"}, fake.routesCalled())
}

// TestScopedRoleGrantToInvite covers the source/warehouse/function/space entitlements,
// which go through grantRoleEntitlement rather than the workspace role builder.
func TestScopedRoleGrantToInvite(t *testing.T) {
	ctx := context.Background()

	fake := newFakeSegment(t)
	fake.respondJSON("GET /roles", http.StatusOK,
		`{"data":{"roles":[{"id":"role_source_admin","name":"Source Admin"},{"id":"`+testRoleID+`","name":"Workspace Admin"}]}}`)
	fake.respondJSON("POST /invites", http.StatusCreated, `{"data":{"emails":["`+testInviteEmail+`"]}}`)

	sourceRes, err := rs.NewResource("Website", sourceResourceType, "source_001", rs.WithParentResourceID(testWorkspaceResourceID()))
	require.NoError(t, err)
	entitlement := ent.NewPermissionEntitlement(sourceRes, "source_admin")

	grants, _, err := grantRoleEntitlement(ctx, fake.client(), testInvitePrincipal(t, testInviteEmail), entitlement)
	require.NoError(t, err)
	require.Len(t, grants, 1)

	assert.Equal(t, []string{"GET /roles", "POST /invites"}, fake.routesCalled())

	bodies := fake.bodiesFor("POST /invites")
	require.Len(t, bodies, 1)
	req := decodeInviteRequest(t, bodies[0])
	require.Len(t, req.Invites, 1)
	require.Len(t, req.Invites[0].Permissions, 1)
	assert.Equal(t, "role_source_admin", req.Invites[0].Permissions[0].RoleID)
	assert.Equal(t,
		[]client.ResourceInput{{ID: "source_001", Type: ResourceTypeSource}},
		req.Invites[0].Permissions[0].Resources,
	)
}

func TestPrincipalEmail(t *testing.T) {
	invite := testInvitePrincipal(t, testInviteEmail)

	email, err := principalEmail(invite)
	require.NoError(t, err)
	assert.Equal(t, testInviteEmail, email)

	// An invite principal stripped of its user trait still resolves: the resource ID is
	// the email address.
	bare := &v2.Resource{Id: &v2.ResourceId{ResourceType: inviteResourceType.Id, Resource: testInviteEmail}}
	email, err = principalEmail(bare)
	require.NoError(t, err)
	assert.Equal(t, testInviteEmail, email)

	user := testUserPrincipal(t, "user_001", "existing@example.com")
	email, err = principalEmail(user)
	require.NoError(t, err)
	assert.Equal(t, "existing@example.com", email)

	_, err = principalEmail(&v2.Resource{Id: &v2.ResourceId{ResourceType: userResourceType.Id, Resource: "user_002"}})
	require.Error(t, err)
}

// TestRoleGrantToInviteReissuesOnValidationDuplicate: Segment answers some duplicate
// invitations with 422 ValidationFailure rather than 409, so the message is matched too.
func TestRoleGrantToInviteReissuesOnValidationDuplicate(t *testing.T) {
	ctx := context.Background()

	fake := newFakeSegment(t)
	fake.respondJSON("POST /invites", http.StatusUnprocessableEntity,
		`{"errors":[{"type":"ValidationFailure","message":"this user has already been invited"}]}`)
	fake.respondJSON("POST /invites", http.StatusCreated, `{"data":{"emails":["`+testInviteEmail+`"]}}`)
	fake.respondJSON("DELETE /invites", http.StatusOK, `{"data":{"status":"SUCCESS"}}`)

	b := newRoleBuilder(fake.client())
	_, _, err := b.Grant(ctx, testInvitePrincipal(t, testInviteEmail), testRoleEntitlement(t))
	require.NoError(t, err)

	assert.Equal(t, []string{"POST /invites", "DELETE /invites", "POST /invites"}, fake.routesCalled())
}

func TestIsAlreadyInvitedError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "conflict", err: status.Error(codes.AlreadyExists, "Request failed with status 409: type: Conflict, message: nope"), want: true},
		{
			name: "validation failure naming a duplicate",
			err:  status.Error(codes.InvalidArgument, "Request failed with status 422: user has ALREADY been invited"),
			want: true,
		},
		{
			name: "unrelated validation failure",
			err:  status.Error(codes.InvalidArgument, "Request failed with status 422: roleId is not valid"),
			want: false,
		},
		{name: "not found", err: status.Error(codes.NotFound, "Request failed with status 404: nope"), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, isAlreadyInvitedError(test.err))
		})
	}
}
