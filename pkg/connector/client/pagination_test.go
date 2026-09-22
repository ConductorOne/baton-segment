package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
)

// clientServing points a client at a stub Segment that always replies with body. Each test
// gets its own server, so the uhttp response cache cannot carry a body between them.
func clientServing(t *testing.T, body string) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	client, err := New(context.Background(), "token", server.URL)
	if err != nil {
		t.Fatalf("Expected no error building the client, got %v", err)
	}

	return client
}

// Segment keeps the pagination object on the last page and omits only next. That is the
// end of the sync, not a failure.
func TestListUsersLastPage(t *testing.T) {
	c := clientServing(t, `{"data":{"users":[{"id":"u1","name":"Ada"}],"pagination":{"current":"MA==","totalEntries":1}}}`)

	res, _, err := c.ListUsers(context.Background(), "", 100)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(res.Data.Users) != 1 {
		t.Fatalf("Expected 1 user, got %d", len(res.Data.Users))
	}
	if res.Data.Pagination.Next != "" {
		t.Errorf("Expected an empty cursor on the last page, got %q", res.Data.Pagination.Next)
	}
}

func TestListUsersNextCursor(t *testing.T) {
	c := clientServing(t, `{"data":{"users":[{"id":"u1"}],"pagination":{"current":"MA==","next":"MTAw"}}}`)

	res, _, err := c.ListUsers(context.Background(), "", 100)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if res.Data.Pagination.Next != "MTAw" {
		t.Errorf("Expected cursor %q, got %q", "MTAw", res.Data.Pagination.Next)
	}
}

// The case the opt-in exists for. Segment's published spec marks pagination required on
// every list output, so a 200 without it means the object was dropped.
func TestListUsersMissingPagination(t *testing.T) {
	c := clientServing(t, `{"data":{"users":[{"id":"u1"}]}}`)

	_, _, err := c.ListUsers(context.Background(), "", 100)
	if err == nil {
		t.Fatal("Expected an error when the pagination object is missing")
	}
	if !errors.Is(err, uhttp.ErrMissingPaginationData) {
		t.Fatalf("Expected ErrMissingPaginationData, got %v", err)
	}
}

func TestListGroupsMissingPagination(t *testing.T) {
	c := clientServing(t, `{"data":{"userGroups":[{"id":"g1","name":"eng"}]}}`)

	_, _, err := c.ListGroups(context.Background(), "", 100)
	if err == nil {
		t.Fatal("Expected an error when the pagination object is missing")
	}
	if !errors.Is(err, uhttp.ErrMissingPaginationData) {
		t.Fatalf("Expected ErrMissingPaginationData, got %v", err)
	}
}

// Single-resource reads share the same request path and carry no pagination object, so
// they must not start demanding one.
func TestGetUserDoesNotRequirePagination(t *testing.T) {
	c := clientServing(t, `{"data":{"user":{"id":"u1","name":"Ada","email":"ada@example.com"}}}`)

	res, _, err := c.GetUser(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if res.Data.User.ID != "u1" {
		t.Errorf("Expected user id %q, got %q", "u1", res.Data.User.ID)
	}
}
