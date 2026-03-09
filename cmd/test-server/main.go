package main

import (
	"log"
	"net/http"
	"time"
)

const (
	// TestAccessToken is the static access token for testing.
	TestAccessToken = "test-segment-token-12345"

	// DefaultPort is the default port for the test server.
	DefaultPort = "8080"
)

func main() {
	ts := NewTestServer()

	mux := http.NewServeMux()

	// Health check (no auth required, used by CI to verify server is up)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck // health check response write failure is non-critical
	})

	// Workspace endpoint (exact root only — "GET /" is a subtree pattern so guard against other paths)
	mux.HandleFunc("GET /", ts.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		ts.handleGetWorkspace(w, r)
	}))

	// Users endpoints
	mux.HandleFunc("GET /users", ts.authMiddleware(ts.handleListUsers))
	mux.HandleFunc("GET /users/{id}", ts.authMiddleware(ts.handleGetUser))
	mux.HandleFunc("DELETE /users", ts.authMiddleware(ts.handleDeleteUser))
	mux.HandleFunc("POST /users/{id}/permissions", ts.authMiddleware(ts.handleAddUserPermissions))
	mux.HandleFunc("PUT /users/{id}/permissions", ts.authMiddleware(ts.handleReplaceUserPermissions))

	// Groups endpoints
	mux.HandleFunc("GET /groups", ts.authMiddleware(ts.handleListGroups))
	mux.HandleFunc("GET /groups/{id}", ts.authMiddleware(ts.handleGetGroup))
	mux.HandleFunc("GET /groups/{id}/users", ts.authMiddleware(ts.handleListGroupUsers))
	mux.HandleFunc("POST /groups/{id}/users", ts.authMiddleware(ts.handleAddUsersToGroup))
	mux.HandleFunc("DELETE /groups/{id}/users", ts.authMiddleware(ts.handleRemoveUsersFromGroup))
	mux.HandleFunc("POST /groups/{id}/permissions", ts.authMiddleware(ts.handleAddGroupPermissions))
	mux.HandleFunc("PUT /groups/{id}/permissions", ts.authMiddleware(ts.handleReplaceGroupPermissions))

	// Roles endpoints
	mux.HandleFunc("GET /roles", ts.authMiddleware(ts.handleListRoles))

	// Invites endpoints
	mux.HandleFunc("GET /invites", ts.authMiddleware(ts.handleListInvites))
	mux.HandleFunc("POST /invites", ts.authMiddleware(ts.handleCreateInvites))
	mux.HandleFunc("DELETE /invites", ts.authMiddleware(ts.handleDeleteInvites))

	// Sources endpoints
	mux.HandleFunc("GET /sources", ts.authMiddleware(ts.handleListSources))

	// Warehouses endpoints
	mux.HandleFunc("GET /warehouses", ts.authMiddleware(ts.handleListWarehouses))

	// Functions endpoints
	mux.HandleFunc("GET /functions", ts.authMiddleware(ts.handleListFunctions))

	// Spaces endpoints
	mux.HandleFunc("GET /spaces", ts.authMiddleware(ts.handleListSpaces))

	log.Printf("🚀 Segment Mock Test Server starting on port %s", DefaultPort)
	log.Printf("📝 Test Access Token: %s", TestAccessToken)
	log.Printf("")
	log.Printf("Available endpoints:")
	log.Printf("  GET    /health                     - Health check (no auth)")
	log.Printf("  GET    /                           - Get workspace info")
	log.Printf("  GET    /users                      - List all users")
	log.Printf("  GET    /users/{id}                 - Get user by ID")
	log.Printf("  DELETE /users                      - Delete user by ID (query param)")
	log.Printf("  POST   /users/{id}/permissions     - Add user permissions")
	log.Printf("  PUT    /users/{id}/permissions     - Replace user permissions")
	log.Printf("  GET    /groups                     - List all groups")
	log.Printf("  GET    /groups/{id}                - Get group by ID")
	log.Printf("  GET    /groups/{id}/users          - List group members")
	log.Printf("  POST   /groups/{id}/users          - Add users to group")
	log.Printf("  DELETE /groups/{id}/users          - Remove users from group")
	log.Printf("  POST   /groups/{id}/permissions    - Add group permissions")
	log.Printf("  PUT    /groups/{id}/permissions    - Replace group permissions")
	log.Printf("  GET    /roles                      - List all roles")
	log.Printf("  GET    /invites                    - List pending invites")
	log.Printf("  POST   /invites                    - Create invites")
	log.Printf("  DELETE /invites                    - Delete invites")
	log.Printf("  GET    /sources                    - List all sources")
	log.Printf("  GET    /warehouses                 - List all warehouses")
	log.Printf("  GET    /functions                  - List all functions")
	log.Printf("  GET    /spaces                     - List all spaces")

	srv := &http.Server{
		Addr:         ":" + DefaultPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Server stopping: %v", srv.ListenAndServe())
}
