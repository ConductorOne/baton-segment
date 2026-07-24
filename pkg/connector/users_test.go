package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/cli"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-segment/pkg/connector/client"
	"github.com/stretchr/testify/require"
)

// newTestUserServer spins up an httptest.Server serving GET /users/{id} with a
// user whose permissions span WORKSPACE (role), SOURCE, WAREHOUSE, FUNCTION,
// and SPACE scoped resources.
func newTestUserServer(t *testing.T) *httptest.Server {
	t.Helper()

	resp := client.GetUserResponse{}
	resp.Data.User = client.User{
		ID:    "u1",
		Name:  "Test User",
		Email: "user@example.com",
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
	mux.HandleFunc("/users/u1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	return httptest.NewServer(mux)
}

func userGrantTargetTypes(t *testing.T, grants []*v2.Grant) []string {
	t.Helper()
	var types []string
	for _, g := range grants {
		types = append(types, g.Entitlement.Resource.Id.ResourceType)
	}
	sort.Strings(types)
	return types
}

func TestUserBuilder_Grants_NoFilter_EmitsAllCrossTypeGrants(t *testing.T) {
	ctx := context.Background()

	server := newTestUserServer(t)
	defer server.Close()

	c, err := client.New(ctx, "test-token", server.URL)
	require.NoError(t, err)

	testCases := []*cli.ConnectorOpts{
		nil,
		{},
		{SyncResourceTypeIDs: nil},
	}

	for _, cliOpts := range testCases {
		b := newUserBuilder(c, cliOpts)

		userResource := &v2.Resource{
			Id: &v2.ResourceId{
				ResourceType: userResourceType.Id,
				Resource:     "u1",
			},
		}

		grants, _, err := b.Grants(ctx, userResource, rs.SyncOpAttrs{})
		require.NoError(t, err)
		require.Len(t, grants, 5)

		got := userGrantTargetTypes(t, grants)
		want := []string{"function", "role", "source", "space", "warehouse"}
		require.Equal(t, want, got)
	}
}

func TestUserBuilder_Grants_Filtered_OnlySyncsRequestedTypes(t *testing.T) {
	ctx := context.Background()

	server := newTestUserServer(t)
	defer server.Close()

	c, err := client.New(ctx, "test-token", server.URL)
	require.NoError(t, err)

	cliOpts := &cli.ConnectorOpts{SyncResourceTypeIDs: []string{"user", "source"}}
	b := newUserBuilder(c, cliOpts)

	userResource := &v2.Resource{
		Id: &v2.ResourceId{
			ResourceType: userResourceType.Id,
			Resource:     "u1",
		},
	}

	grants, _, err := b.Grants(ctx, userResource, rs.SyncOpAttrs{})
	require.NoError(t, err)
	require.Len(t, grants, 1)

	got := userGrantTargetTypes(t, grants)
	require.Equal(t, []string{"source"}, got)
}
