package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

const (
	// defaultCursor is the base64-encoded default cursor for first page pagination.
	// This matches the Segment API behavior.
	defaultCursor = "MA=="

	dataKey        = "data"
	paginationKey  = "pagination"
	statusSuccess  = "SUCCESS"
	groupNameKey   = "name"
	memberCountKey = "memberCount"
	permissionsKey = "permissions"
	statusKey      = "status"
)

// Pagination represents pagination information.
type Pagination struct {
	Current      string `json:"current,omitempty"`
	Next         string `json:"next,omitempty"`
	Previous     string `json:"previous,omitempty"`
	TotalEntries int    `json:"totalEntries,omitempty"`
}

// authMiddleware validates the access token.
func (ts *TestServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		if authHeader == "" {
			logf("❌ Missing Authorization header")
			http.Error(w, `{"error":"unauthorized","message":"Authorization header required"}`, http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			logf("❌ Invalid Authorization header format")
			http.Error(w, `{"error":"unauthorized","message":"Bearer token required"}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != TestAccessToken {
			logf("❌ Invalid access token: %s...", token[:min(10, len(token))])
			http.Error(w, `{"error":"unauthorized","message":"Invalid access token"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// handleListUsers returns all users with pagination.
func (ts *TestServer) handleListUsers(w http.ResponseWriter, r *http.Request) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	pageSize, cursor := parsePagination(r)

	// Convert map to sorted slice
	var users []User
	for _, u := range ts.users {
		users = append(users, u)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].ID < users[j].ID })

	// Apply pagination
	startIdx := findStartIndex(cursor, users, func(u User) string { return u.ID })
	endIdx := startIdx + pageSize
	if endIdx > len(users) {
		endIdx = len(users)
	}

	pageUsers := users[startIdx:endIdx]

	var nextCursor string
	if endIdx < len(users) {
		nextCursor = users[endIdx].ID
	}

	// Build current cursor (base64 encoded for consistency with real API)
	currentCursor := cursor
	if currentCursor == "" {
		currentCursor = defaultCursor
	}

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			"users": pageUsers,
			paginationKey: Pagination{
				Current:      currentCursor,
				Next:         nextCursor,
				TotalEntries: len(users),
			},
		},
	}

	logf("✅ GET /users - returned %d users (cursor: %s, next: %s)", len(pageUsers), cursor, nextCursor)
	sendJSON(w, response)
}

// handleGetUser returns a single user by ID.
func (ts *TestServer) handleGetUser(w http.ResponseWriter, r *http.Request) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	userID := r.PathValue("id")

	user, exists := ts.users[userID]
	if !exists {
		logf("❌ GET /users/%s - not found", userID)
		http.Error(w, `{"error":"not_found","message":"User not found"}`, http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			"user": user,
		},
	}

	logf("✅ GET /users/%s - found user: %s", userID, user.Email)
	sendJSON(w, response)
}

// handleDeleteUser removes a user from the workspace.
func (ts *TestServer) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	userID := r.URL.Query().Get("userIds.0")
	if userID == "" {
		logf("❌ DELETE /users - missing userIds.0 parameter")
		http.Error(w, `{"error":"bad_request","message":"userIds.0 is required"}`, http.StatusBadRequest)
		return
	}

	if _, exists := ts.users[userID]; !exists {
		logf("❌ DELETE /users - user %s not found", userID)
		http.Error(w, `{"error":"not_found","message":"User not found"}`, http.StatusNotFound)
		return
	}

	// Remove user from all groups
	for groupID, group := range ts.groups {
		group.Members = removeFromSlice(group.Members, userID)
		ts.groups[groupID] = group
		ts.updateGroupMemberCount(groupID)
	}

	delete(ts.users, userID)

	logf("✅ DELETE /users - deleted user: %s", userID)
	sendJSON(w, map[string]interface{}{dataKey: map[string]string{statusKey: statusSuccess}})
}

// handleListGroups returns all groups with pagination.
func (ts *TestServer) handleListGroups(w http.ResponseWriter, r *http.Request) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	pageSize, cursor := parsePagination(r)

	// Convert map to sorted slice (without members for list response)
	var groups []map[string]interface{}
	var sortedGroups []Group
	for _, g := range ts.groups {
		sortedGroups = append(sortedGroups, g)
	}
	sort.Slice(sortedGroups, func(i, j int) bool { return sortedGroups[i].ID < sortedGroups[j].ID })

	for _, g := range sortedGroups {
		groups = append(groups, map[string]interface{}{
			"id":          g.ID,
			groupNameKey:   g.Name,
			memberCountKey: g.MemberCount,
		})
	}

	// Apply pagination
	startIdx := 0
	if cursor != "" {
		for i, g := range sortedGroups {
			if g.ID == cursor {
				startIdx = i
				break
			}
		}
	}
	endIdx := startIdx + pageSize
	if endIdx > len(groups) {
		endIdx = len(groups)
	}

	pageGroups := groups[startIdx:endIdx]

	var nextCursor string
	if endIdx < len(groups) {
		nextCursor = sortedGroups[endIdx].ID
	}

	currentCursor := cursor
	if currentCursor == "" {
		currentCursor = defaultCursor
	}

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			"userGroups": pageGroups,
			paginationKey: Pagination{
				Current:      currentCursor,
				Next:         nextCursor,
				TotalEntries: len(groups),
			},
		},
	}

	logf("✅ GET /groups - returned %d groups", len(pageGroups))
	sendJSON(w, response)
}

// handleGetGroup returns a single group by ID with permissions.
func (ts *TestServer) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	groupID := r.PathValue("id")

	group, exists := ts.groups[groupID]
	if !exists {
		logf("❌ GET /groups/%s - not found", groupID)
		http.Error(w, `{"error":"not_found","message":"Group not found"}`, http.StatusNotFound)
		return
	}

	// Return group with permissions but without member list
	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			"userGroup": map[string]interface{}{
				"id":          group.ID,
				groupNameKey:   group.Name,
				memberCountKey: group.MemberCount,
				permissionsKey: group.Permissions,
			},
		},
	}

	logf("✅ GET /groups/%s - found group: %s", groupID, group.Name)
	sendJSON(w, response)
}

// handleListGroupUsers returns users in a group with pagination.
func (ts *TestServer) handleListGroupUsers(w http.ResponseWriter, r *http.Request) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	groupID := r.PathValue("id")
	pageSize, cursor := parsePagination(r)

	group, exists := ts.groups[groupID]
	if !exists {
		logf("❌ GET /groups/%s/users - group not found", groupID)
		http.Error(w, `{"error":"not_found","message":"Group not found"}`, http.StatusNotFound)
		return
	}

	// Get users in the group
	var users []User
	for _, memberID := range group.Members {
		if user, exists := ts.users[memberID]; exists {
			users = append(users, user)
		}
	}
	sort.Slice(users, func(i, j int) bool { return users[i].ID < users[j].ID })

	// Apply pagination
	startIdx := findStartIndex(cursor, users, func(u User) string { return u.ID })
	endIdx := startIdx + pageSize
	if endIdx > len(users) {
		endIdx = len(users)
	}

	pageUsers := users[startIdx:endIdx]

	var nextCursor string
	if endIdx < len(users) {
		nextCursor = users[endIdx].ID
	}

	currentCursor := cursor
	if currentCursor == "" {
		currentCursor = defaultCursor
	}

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			"users": pageUsers,
			paginationKey: Pagination{
				Current:      currentCursor,
				Next:         nextCursor,
				TotalEntries: len(users),
			},
		},
	}

	logf("✅ GET /groups/%s/users - returned %d users", groupID, len(pageUsers))
	sendJSON(w, response)
}

// handleAddUsersToGroup adds users to a group by email.
func (ts *TestServer) handleAddUsersToGroup(w http.ResponseWriter, r *http.Request) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	groupID := r.PathValue("id")

	var req struct {
		Emails []string `json:"emails"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logf("❌ POST /groups/%s/users - invalid request body", groupID)
		http.Error(w, `{"error":"bad_request","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	group, exists := ts.groups[groupID]
	if !exists {
		logf("❌ POST /groups/%s/users - group not found", groupID)
		http.Error(w, `{"error":"not_found","message":"Group not found"}`, http.StatusNotFound)
		return
	}

	for _, email := range req.Emails {
		user := ts.getUserByEmailLocked(email)
		if user == nil {
			logf("⚠️ POST /groups/%s/users - user with email %s not found", groupID, email)
			continue
		}

		if !contains(group.Members, user.ID) {
			group.Members = append(group.Members, user.ID)
		}
	}

	group.MemberCount = len(group.Members)
	ts.groups[groupID] = group

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			"userGroup": map[string]interface{}{
				"id":          group.ID,
				groupNameKey:   group.Name,
				memberCountKey: group.MemberCount,
			},
		},
	}

	logf("✅ POST /groups/%s/users - added users: %v", groupID, req.Emails)
	sendJSON(w, response)
}

// handleRemoveUsersFromGroup removes users from a group by email.
func (ts *TestServer) handleRemoveUsersFromGroup(w http.ResponseWriter, r *http.Request) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	groupID := r.PathValue("id")

	// Parse emails from query parameters
	var emails []string
	for i := 0; ; i++ {
		email := r.URL.Query().Get("emails." + strconv.Itoa(i))
		if email == "" {
			break
		}
		emails = append(emails, email)
	}

	group, exists := ts.groups[groupID]
	if !exists {
		logf("❌ DELETE /groups/%s/users - group not found", groupID)
		http.Error(w, `{"error":"not_found","message":"Group not found"}`, http.StatusNotFound)
		return
	}

	for _, email := range emails {
		user := ts.getUserByEmailLocked(email)
		if user != nil {
			group.Members = removeFromSlice(group.Members, user.ID)
		}
	}

	group.MemberCount = len(group.Members)
	ts.groups[groupID] = group

	logf("✅ DELETE /groups/%s/users - removed users: %v", groupID, emails)
	sendJSON(w, map[string]interface{}{dataKey: map[string]string{statusKey: statusSuccess}})
}

// handleListRoles returns all roles.
// Roles are system-defined by Segment (~8 built-in), so no pagination is needed.
func (ts *TestServer) handleListRoles(w http.ResponseWriter, r *http.Request) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var roles []Role
	for _, role := range ts.roles {
		roles = append(roles, role)
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].ID < roles[j].ID })

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			"roles": roles,
		},
	}

	logf("✅ GET /roles - returned %d roles", len(roles))
	sendJSON(w, response)
}

// handleListInvites returns pending invitations with pagination.
func (ts *TestServer) handleListInvites(w http.ResponseWriter, r *http.Request) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	pageSize, cursor := parsePagination(r)

	var invites []string
	for email := range ts.invites {
		invites = append(invites, email)
	}
	sort.Strings(invites)

	// Apply pagination
	startIdx := 0
	if cursor != "" {
		for i, email := range invites {
			if email == cursor {
				startIdx = i
				break
			}
		}
	}
	endIdx := startIdx + pageSize
	if endIdx > len(invites) {
		endIdx = len(invites)
	}

	pageInvites := invites[startIdx:endIdx]

	var nextCursor string
	if endIdx < len(invites) {
		nextCursor = invites[endIdx]
	}

	currentCursor := cursor
	if currentCursor == "" {
		currentCursor = defaultCursor
	}

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			"invites": pageInvites,
			paginationKey: Pagination{
				Current:      currentCursor,
				Next:         nextCursor,
				TotalEntries: len(invites),
			},
		},
	}

	logf("✅ GET /invites - returned %d invites", len(pageInvites))
	sendJSON(w, response)
}

// findInviteKey returns the stored map key for a case-insensitive email
// match, preserving whatever casing the invite was originally created with.
func (ts *TestServer) findInviteKey(email string) (string, bool) {
	for existing := range ts.invites {
		if strings.EqualFold(existing, email) {
			return existing, true
		}
	}
	return "", false
}

// handleCreateInvites creates new workspace invitations.
func (ts *TestServer) handleCreateInvites(w http.ResponseWriter, r *http.Request) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	var req struct {
		Invites []struct {
			Email string `json:"email"`
		} `json:"invites"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logf("❌ POST /invites - invalid request body")
		http.Error(w, `{"error":"bad_request","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	for _, invite := range req.Invites {
		if _, found := ts.findInviteKey(invite.Email); found {
			// Verified against the live Segment API.
			logf("❌ POST /invites - already invited: %s", invite.Email)
			sendJSONError(w, http.StatusBadRequest, `{"errors":[{"type":"bad-request","message":"One or more email address was already invited to join workspace."}]}`)
			return
		}
		for _, user := range ts.users {
			if strings.EqualFold(user.Email, invite.Email) {
				// Verified against the live Segment API: identical body to the
				// already-invited case above - Segment does not distinguish them.
				logf("❌ POST /invites - already a member: %s", invite.Email)
				sendJSONError(w, http.StatusBadRequest, `{"errors":[{"type":"bad-request","message":"One or more email address was already invited to join workspace."}]}`)
				return
			}
		}
	}

	var emails []string
	for _, invite := range req.Invites {
		// Preserve the caller's casing: List() returns these keys verbatim,
		// and CreateAccount's resource ID is built from the same input, so
		// normalizing here would make the two diverge for a mixed-case email.
		ts.invites[invite.Email] = true
		emails = append(emails, invite.Email)
	}

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			"emails": emails,
		},
	}

	logf("✅ POST /invites - created invites: %v", emails)
	sendJSON(w, response)
}

// handleDeleteInvites removes pending invitations by email.
func (ts *TestServer) handleDeleteInvites(w http.ResponseWriter, r *http.Request) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Parse emails from query parameters
	var emails []string
	for i := 0; ; i++ {
		email := r.URL.Query().Get("emails." + strconv.Itoa(i))
		if email == "" {
			break
		}
		emails = append(emails, email)
	}

	for _, email := range emails {
		if key, found := ts.findInviteKey(email); found {
			delete(ts.invites, key)
		}
	}

	logf("✅ DELETE /invites - deleted invites: %v", emails)
	sendJSON(w, map[string]interface{}{dataKey: map[string]string{statusKey: statusSuccess}})
}

// handleGetWorkspace returns the current workspace.
func (ts *TestServer) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			"workspace": ts.workspace,
		},
	}

	logf("✅ GET / - returned workspace: %s", ts.workspace.Name)
	sendJSON(w, response)
}

// handleListSources returns all sources with pagination.
func (ts *TestServer) handleListSources(w http.ResponseWriter, r *http.Request) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	pageSize, cursor := parsePagination(r)

	// Convert map to sorted slice
	var sources []Source
	for _, s := range ts.sources {
		sources = append(sources, s)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].ID < sources[j].ID })

	// Apply pagination
	startIdx := findStartIndex(cursor, sources, func(s Source) string { return s.ID })
	endIdx := startIdx + pageSize
	if endIdx > len(sources) {
		endIdx = len(sources)
	}

	pageSources := sources[startIdx:endIdx]

	var nextCursor string
	if endIdx < len(sources) {
		nextCursor = sources[endIdx].ID
	}

	currentCursor := cursor
	if currentCursor == "" {
		currentCursor = defaultCursor
	}

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			"sources": pageSources,
			paginationKey: Pagination{
				Current:      currentCursor,
				Next:         nextCursor,
				TotalEntries: len(sources),
			},
		},
	}

	logf("✅ GET /sources - returned %d sources", len(pageSources))
	sendJSON(w, response)
}

// handleListWarehouses returns all warehouses with pagination.
func (ts *TestServer) handleListWarehouses(w http.ResponseWriter, r *http.Request) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	pageSize, cursor := parsePagination(r)

	// Convert map to sorted slice
	var warehouses []Warehouse
	for _, wh := range ts.warehouses {
		warehouses = append(warehouses, wh)
	}
	sort.Slice(warehouses, func(i, j int) bool { return warehouses[i].ID < warehouses[j].ID })

	// Apply pagination
	startIdx := findStartIndex(cursor, warehouses, func(wh Warehouse) string { return wh.ID })
	endIdx := startIdx + pageSize
	if endIdx > len(warehouses) {
		endIdx = len(warehouses)
	}

	pageWarehouses := warehouses[startIdx:endIdx]

	var nextCursor string
	if endIdx < len(warehouses) {
		nextCursor = warehouses[endIdx].ID
	}

	currentCursor := cursor
	if currentCursor == "" {
		currentCursor = defaultCursor
	}

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			"warehouses": pageWarehouses,
			paginationKey: Pagination{
				Current:      currentCursor,
				Next:         nextCursor,
				TotalEntries: len(warehouses),
			},
		},
	}

	logf("✅ GET /warehouses - returned %d warehouses", len(pageWarehouses))
	sendJSON(w, response)
}

// handleListFunctions returns all functions with pagination.
func (ts *TestServer) handleListFunctions(w http.ResponseWriter, r *http.Request) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	pageSize, cursor := parsePagination(r)
	resourceType := r.URL.Query().Get("resourceType")

	// Convert map to sorted slice, optionally filtering by resource type
	var functions []Function
	for _, fn := range ts.functions {
		if resourceType == "" || fn.ResourceType == resourceType {
			functions = append(functions, fn)
		}
	}
	sort.Slice(functions, func(i, j int) bool { return functions[i].ID < functions[j].ID })

	// Apply pagination
	startIdx := findStartIndex(cursor, functions, func(fn Function) string { return fn.ID })
	endIdx := startIdx + pageSize
	if endIdx > len(functions) {
		endIdx = len(functions)
	}

	pageFunctions := functions[startIdx:endIdx]

	var nextCursor string
	if endIdx < len(functions) {
		nextCursor = functions[endIdx].ID
	}

	currentCursor := cursor
	if currentCursor == "" {
		currentCursor = defaultCursor
	}

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			"functions": pageFunctions,
			paginationKey: Pagination{
				Current:      currentCursor,
				Next:         nextCursor,
				TotalEntries: len(functions),
			},
		},
	}

	logf("✅ GET /functions - returned %d functions", len(pageFunctions))
	sendJSON(w, response)
}

// handleListSpaces returns all spaces with pagination.
func (ts *TestServer) handleListSpaces(w http.ResponseWriter, r *http.Request) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	pageSize, cursor := parsePagination(r)

	// Convert map to sorted slice
	var spaces []Space
	for _, sp := range ts.spaces {
		spaces = append(spaces, sp)
	}
	sort.Slice(spaces, func(i, j int) bool { return spaces[i].ID < spaces[j].ID })

	// Apply pagination
	startIdx := findStartIndex(cursor, spaces, func(sp Space) string { return sp.ID })
	endIdx := startIdx + pageSize
	if endIdx > len(spaces) {
		endIdx = len(spaces)
	}

	pageSpaces := spaces[startIdx:endIdx]

	var nextCursor string
	if endIdx < len(spaces) {
		nextCursor = spaces[endIdx].ID
	}

	currentCursor := cursor
	if currentCursor == "" {
		currentCursor = defaultCursor
	}

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			"spaces": pageSpaces,
			paginationKey: Pagination{
				Current:      currentCursor,
				Next:         nextCursor,
				TotalEntries: len(spaces),
			},
		},
	}

	logf("✅ GET /spaces - returned %d spaces", len(pageSpaces))
	sendJSON(w, response)
}

// handleAddUserPermissions appends permissions to a user (POST).
func (ts *TestServer) handleAddUserPermissions(w http.ResponseWriter, r *http.Request) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	userID := r.PathValue("id")

	var req struct {
		Permissions []Permission `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logf("❌ POST /users/%s/permissions - invalid request body", userID)
		http.Error(w, `{"error":"bad_request","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	user, exists := ts.users[userID]
	if !exists {
		logf("❌ POST /users/%s/permissions - user not found", userID)
		http.Error(w, `{"error":"not_found","message":"User not found"}`, http.StatusNotFound)
		return
	}

	// Add role names and append to existing permissions
	for i := range req.Permissions {
		if role, ok := ts.roles[req.Permissions[i].RoleID]; ok {
			req.Permissions[i].RoleName = role.Name
		}
	}

	user.Permissions = append(user.Permissions, req.Permissions...)
	ts.users[userID] = user

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			permissionsKey: user.Permissions,
		},
	}

	logf("✅ POST /users/%s/permissions - added %d permissions (total: %d)", userID, len(req.Permissions), len(user.Permissions))
	sendJSON(w, response)
}

// handleReplaceUserPermissions replaces all permissions for a user (PUT).
func (ts *TestServer) handleReplaceUserPermissions(w http.ResponseWriter, r *http.Request) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	userID := r.PathValue("id")

	var req struct {
		Permissions []Permission `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logf("❌ PUT /users/%s/permissions - invalid request body", userID)
		http.Error(w, `{"error":"bad_request","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	user, exists := ts.users[userID]
	if !exists {
		logf("❌ PUT /users/%s/permissions - user not found", userID)
		http.Error(w, `{"error":"not_found","message":"User not found"}`, http.StatusNotFound)
		return
	}

	// Add role names to permissions
	for i := range req.Permissions {
		if role, ok := ts.roles[req.Permissions[i].RoleID]; ok {
			req.Permissions[i].RoleName = role.Name
		}
	}

	user.Permissions = req.Permissions
	ts.users[userID] = user

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			permissionsKey: user.Permissions,
		},
	}

	logf("✅ PUT /users/%s/permissions - replaced permissions (%d)", userID, len(req.Permissions))
	sendJSON(w, response)
}

// handleAddGroupPermissions appends permissions to a group (POST).
func (ts *TestServer) handleAddGroupPermissions(w http.ResponseWriter, r *http.Request) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	groupID := r.PathValue("id")

	var req struct {
		Permissions []Permission `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logf("❌ POST /groups/%s/permissions - invalid request body", groupID)
		http.Error(w, `{"error":"bad_request","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	group, exists := ts.groups[groupID]
	if !exists {
		logf("❌ POST /groups/%s/permissions - group not found", groupID)
		http.Error(w, `{"error":"not_found","message":"Group not found"}`, http.StatusNotFound)
		return
	}

	// Add role names and append to existing permissions
	for i := range req.Permissions {
		if role, ok := ts.roles[req.Permissions[i].RoleID]; ok {
			req.Permissions[i].RoleName = role.Name
		}
	}

	group.Permissions = append(group.Permissions, req.Permissions...)
	ts.groups[groupID] = group

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			permissionsKey: group.Permissions,
		},
	}

	logf("✅ POST /groups/%s/permissions - added %d permissions (total: %d)", groupID, len(req.Permissions), len(group.Permissions))
	sendJSON(w, response)
}

// handleReplaceGroupPermissions replaces all permissions for a group (PUT).
func (ts *TestServer) handleReplaceGroupPermissions(w http.ResponseWriter, r *http.Request) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	groupID := r.PathValue("id")

	var req struct {
		Permissions []Permission `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logf("❌ PUT /groups/%s/permissions - invalid request body", groupID)
		http.Error(w, `{"error":"bad_request","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	group, exists := ts.groups[groupID]
	if !exists {
		logf("❌ PUT /groups/%s/permissions - group not found", groupID)
		http.Error(w, `{"error":"not_found","message":"Group not found"}`, http.StatusNotFound)
		return
	}

	// Add role names to permissions
	for i := range req.Permissions {
		if role, ok := ts.roles[req.Permissions[i].RoleID]; ok {
			req.Permissions[i].RoleName = role.Name
		}
	}

	group.Permissions = req.Permissions
	ts.groups[groupID] = group

	response := map[string]interface{}{
		dataKey: map[string]interface{}{
			permissionsKey: group.Permissions,
		},
	}

	logf("✅ PUT /groups/%s/permissions - replaced permissions (%d)", groupID, len(req.Permissions))
	sendJSON(w, response)
}

// Helper functions

func parsePagination(r *http.Request) (int, string) {
	pageSize := 100
	if ps := r.URL.Query().Get("pagination.count"); ps != "" {
		if size, err := strconv.Atoi(ps); err == nil && size > 0 {
			pageSize = size
		}
	}
	cursor := r.URL.Query().Get("pagination.cursor")
	return pageSize, cursor
}

func findStartIndex[T any](cursor string, items []T, getID func(T) string) int {
	if cursor == "" {
		return 0
	}
	for i, item := range items {
		if getID(item) == cursor {
			return i
		}
	}
	return 0
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

func sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/vnd.segment.v1+json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logf("❌ Failed to encode JSON response: %v", err)
	}
}

// sendJSONError writes a JSON error body with the real content type, unlike
// http.Error (which forces text/plain and so bypasses uhttp's JSON error
// decode path on the client).
func sendJSONError(w http.ResponseWriter, statusCode int, body string) {
	w.Header().Set("Content-Type", "application/vnd.segment.v1+json")
	w.WriteHeader(statusCode)
	if _, err := w.Write([]byte(body)); err != nil {
		logf("❌ Failed to write JSON error response: %v", err)
	}
}

// logf wraps log.Printf with input sanitization to prevent log injection (gosec G706).
func logf(format string, args ...interface{}) {
	sanitized := make([]interface{}, len(args))
	copy(sanitized, args)
	for i, arg := range sanitized {
		if s, ok := arg.(string); ok {
			sanitized[i] = strings.NewReplacer("\n", "", "\r", "").Replace(s)
		}
	}
	log.Printf(format, sanitized...) //nolint:gosec // G706 false positive: string args are sanitized above
}
