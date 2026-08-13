package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/cli"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-segment/pkg/connector/client"
	"github.com/stretchr/testify/require"
)

// newTestGroupServer spins up an httptest.Server serving:
//   - GET /groups/{id}/users -> a single page of group members (used by the
//     "group-members" pagination phase).
//   - GET /groups/{id} -> group details with permissions spanning WORKSPACE
//     (role), SOURCE, WAREHOUSE, FUNCTION, and SPACE (used by the
//     "group-roles" pagination phase).
func newTestGroupServer(t *testing.T) *httptest.Server {
	t.Helper()

	membersResp := client.ListGroupUsersResponse{}
	membersResp.Data.Users = []client.User{
		{ID: "u1", Name: "Member One", Email: "member1@example.com"},
	}
	membersResp.Data.Pagination.Next = ""

	groupResp := client.GetGroupResponse{}
	groupResp.Data.UserGroup = client.Group{
		ID:   "g1",
		Name: "Test Group",
		Permissions: []client.Permission{
			{
				RoleID:   "role1",
				RoleName: "Workspace Owner",
				Resources: []client.Resource{
					{ID: "ws1", Type: ResourceTypeWorkspace},
				},
			},
			{
				RoleID:   "role2",
				RoleName: "Source Admin",
				Resources: []client.Resource{
					{ID: "src1", Type: ResourceTypeSource},
				},
			},
			{
				RoleID:   "role3",
				RoleName: "Warehouse Admin",
				Resources: []client.Resource{
					{ID: "wh1", Type: ResourceTypeWarehouse},
				},
			},
			{
				RoleID:   "role4",
				RoleName: "Function Admin",
				Resources: []client.Resource{
					{ID: "fn1", Type: ResourceTypeFunction},
				},
			},
			{
				RoleID:   "role5",
				RoleName: "Space Admin",
				Resources: []client.Resource{
					{ID: "sp1", Type: ResourceTypeSpace},
				},
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/groups/g1/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(membersResp)
	})
	mux.HandleFunc("/groups/g1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(groupResp)
	})

	return httptest.NewServer(mux)
}

func groupRoleGrantTargetTypes(t *testing.T, grants []*v2.Grant) []string {
	t.Helper()
	var types []string
	for _, g := range grants {
		// Group-members phase grants target the "user" resource type; only
		// count cross-type (group-roles phase) grants.
		if g.Entitlement.Resource.Id.ResourceType == userResourceType.Id {
			continue
		}
		types = append(types, g.Entitlement.Resource.Id.ResourceType)
	}
	sort.Strings(types)
	return types
}

// driveGroupRolesPhase runs the group-members phase (to obtain the pagination
// token for group-roles), then the group-roles phase, returning only the
// group-roles grants.
func driveGroupRolesPhase(t *testing.T, ctx context.Context, b *groupBuilder, groupResource *v2.Resource) []*v2.Grant {
	t.Helper()

	// Phase 1: group-members.
	_, results1, err := b.Grants(ctx, groupResource, rs.SyncOpAttrs{})
	require.NoError(t, err)
	require.NotNil(t, results1)
	require.NotEmpty(t, results1.NextPageToken, "expected a pagination token leading into the group-roles phase")

	// Phase 2: group-roles.
	grants2, _, err := b.Grants(ctx, groupResource, rs.SyncOpAttrs{
		PageToken: pagination.Token{Token: results1.NextPageToken},
	})
	require.NoError(t, err)

	return grants2
}

func TestGroupBuilder_Grants_NoFilter_EmitsAllCrossTypeGrants(t *testing.T) {
	ctx := context.Background()

	server := newTestGroupServer(t)
	defer server.Close()

	c, err := client.New(ctx, "test-token", server.URL)
	require.NoError(t, err)

	testCases := []*cli.ConnectorOpts{
		nil,
		{},
		{SyncResourceTypeIDs: nil},
	}

	for _, cliOpts := range testCases {
		b := newGroupBuilder(c, newSkipCrossTypeGrants(cliOpts))

		groupResource := &v2.Resource{
			Id: &v2.ResourceId{
				ResourceType: groupResourceType.Id,
				Resource:     "g1",
			},
		}

		grants := driveGroupRolesPhase(t, ctx, b, groupResource)
		got := groupRoleGrantTargetTypes(t, grants)
		want := []string{"function", "role", "source", "space", "warehouse"}
		require.Equal(t, want, got)
	}
}

func TestGroupBuilder_Grants_Filtered_OnlySyncsRequestedTypes(t *testing.T) {
	ctx := context.Background()

	server := newTestGroupServer(t)
	defer server.Close()

	c, err := client.New(ctx, "test-token", server.URL)
	require.NoError(t, err)

	cliOpts := &cli.ConnectorOpts{SyncResourceTypeIDs: []string{"group", "source"}}
	b := newGroupBuilder(c, newSkipCrossTypeGrants(cliOpts))

	groupResource := &v2.Resource{
		Id: &v2.ResourceId{
			ResourceType: groupResourceType.Id,
			Resource:     "g1",
		},
	}

	grants := driveGroupRolesPhase(t, ctx, b, groupResource)
	got := groupRoleGrantTargetTypes(t, grants)
	require.Equal(t, []string{"source"}, got)
}

// TestGroupBuilder_AllTargetsFiltered_KeepsOwnMemberGrants pins the regression
// that SkipEntitlementsAndGrants on the group resource type introduced: it made
// the SDK skip Grants entirely, which also dropped the group's own member
// grants — data no resource-type filter should affect.
func TestGroupBuilder_AllTargetsFiltered_KeepsOwnMemberGrants(t *testing.T) {
	ctx := context.Background()

	server := newTestGroupServer(t)
	defer server.Close()

	c, err := client.New(ctx, "test-token", server.URL)
	require.NoError(t, err)

	// "group" only: every cross-type target is excluded, so skipTargets.all().
	cliOpts := &cli.ConnectorOpts{SyncResourceTypeIDs: []string{"group"}}
	b := newGroupBuilder(c, newSkipCrossTypeGrants(cliOpts))

	annos := annotations.Annotations(b.ResourceType(ctx).GetAnnotations())
	require.False(t, annos.Contains(&v2.SkipEntitlementsAndGrants{}),
		"group must not carry SkipEntitlementsAndGrants: Grants also emits its own member grants")
	require.True(t, annos.Contains(&v2.SkipEntitlements{}),
		"group's member entitlement comes from StaticEntitlements, so SkipEntitlements still applies")

	groupResource := &v2.Resource{
		Id: &v2.ResourceId{ResourceType: groupResourceType.Id, Resource: "g1"},
	}

	// Phase 1 (group-members) must still emit the group's own member grants.
	memberGrants, results, err := b.Grants(ctx, groupResource, rs.SyncOpAttrs{})
	require.NoError(t, err)
	require.NotNil(t, results)
	require.NotEmpty(t, memberGrants, "group member grants must survive cross-type filtering")
	for _, g := range memberGrants {
		// The group's own member entitlement, granted to a user principal.
		require.Equal(t, groupResourceType.Id, g.Entitlement.Resource.Id.ResourceType)
		require.Equal(t, userResourceType.Id, g.Principal.Id.ResourceType)
	}

	// ...while the cross-type grants are all filtered out.
	grants := driveGroupRolesPhase(t, ctx, b, groupResource)
	require.Empty(t, groupRoleGrantTargetTypes(t, grants))
}

// TestGroupBuilder_AllTargetsFiltered_SkipsGroupFetch pins that the group-roles
// phase does not fetch the group at all when every cross-type target is
// excluded — otherwise it pays one GetGroup per group and discards every grant.
func TestGroupBuilder_AllTargetsFiltered_SkipsGroupFetch(t *testing.T) {
	ctx := context.Background()

	var groupFetches int
	base := newTestGroupServer(t)
	defer base.Close()

	// Wrap the fixture server so GET /groups/{id} can be counted. The members
	// path ends in /users, so a suffix check distinguishes the two.
	counting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/groups/g1" {
			groupFetches++
		}
		http.Redirect(w, r, base.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer counting.Close()

	groupResource := &v2.Resource{
		Id: &v2.ResourceId{ResourceType: groupResourceType.Id, Resource: "g1"},
	}

	// A separate client per case: uhttp caches responses, so sharing one would
	// let the first case's GetGroup satisfy the second without a request.
	newBuilder := func(syncTypes []string) *groupBuilder {
		c, err := client.New(ctx, "test-token", counting.URL)
		require.NoError(t, err)
		return newGroupBuilder(c, newSkipCrossTypeGrants(&cli.ConnectorOpts{SyncResourceTypeIDs: syncTypes}))
	}

	// A target still in scope: the fetch must happen.
	driveGroupRolesPhase(t, ctx, newBuilder([]string{"user", "group", "source"}), groupResource)
	require.Equal(t, 1, groupFetches, "group-roles phase should fetch the group when a target is in scope")

	// Every target excluded: no fetch.
	groupFetches = 0
	driveGroupRolesPhase(t, ctx, newBuilder([]string{"group"}), groupResource)
	require.Zero(t, groupFetches, "group-roles phase should not fetch the group when every target is excluded")
}
