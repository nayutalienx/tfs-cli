package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestGetWorkItemCommentsPaginatesByRevision(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/RND/_apis/wit/workItems/42/comments" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("api-version"); got != workItemCommentsAPIVersion {
			t.Fatalf("unexpected API version: %s", got)
		}
		if got := r.URL.Query().Get("$top"); got != strconv.Itoa(workItemCommentsPageSize) {
			t.Fatalf("unexpected page size: %s", got)
		}
		if got := r.URL.Query().Get("order"); got != "asc" {
			t.Fatalf("unexpected order: %s", got)
		}

		switch r.URL.Query().Get("fromRevision") {
		case "1":
			fmt.Fprint(w, `{"totalCount":3,"count":2,"value":[{"revision":2,"text":"first"},{"revision":5,"text":"second"}]}`)
		case "6":
			fmt.Fprint(w, `{"totalCount":3,"count":1,"value":[{"revision":8,"text":"third"}]}`)
		default:
			t.Fatalf("unexpected fromRevision: %s", r.URL.Query().Get("fromRevision"))
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "RND", "test-pat", false, false, nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	comments, err := client.GetWorkItemComments(context.Background(), 42, 0)
	if err != nil {
		t.Fatalf("GetWorkItemComments returned error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("got %d requests, want 2", requests)
	}
	if len(comments) != 3 {
		t.Fatalf("got %d comments, want 3", len(comments))
	}
	if comments[0].Revision != 2 || comments[1].Revision != 5 || comments[2].Revision != 8 {
		t.Fatalf("unexpected comments: %#v", comments)
	}
}

func TestGetWorkItemCommentsSupportsCommentsPropertyAndLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("$top"); got != "1" {
			t.Fatalf("unexpected page size: %s", got)
		}
		fmt.Fprint(w, `{"totalCount":2,"count":1,"comments":[{"revision":3,"text":"limited"}]}`)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "RND", "test-pat", false, false, nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	comments, err := client.GetWorkItemComments(context.Background(), 42, 1)
	if err != nil {
		t.Fatalf("GetWorkItemComments returned error: %v", err)
	}
	if len(comments) != 1 || comments[0].Text != "limited" {
		t.Fatalf("unexpected comments: %#v", comments)
	}
}
