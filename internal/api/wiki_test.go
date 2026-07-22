package api

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetWikiPageByIDIncludesContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/RND/_apis/wiki/wikis/RND.wiki/pages/1578" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("api-version"); got != wikiAPIVersion {
			t.Fatalf("unexpected API version: %s", got)
		}
		if got := r.URL.Query().Get("includeContent"); got != "true" {
			t.Fatalf("includeContent: got %q, want true", got)
		}
		wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":test-pat"))
		if got := r.Header.Get("Authorization"); got != wantAuth {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":1578,"path":"/Source wallet check","gitItemPath":"/Source-wallet-check.md","content":"# Source wallet check\n\nRules.","remoteUrl":"https://tfs.example/RND/_wiki/wikis/RND.wiki/1578/Source-wallet-check"}`)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "RND", "test-pat", false, false, nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	page, err := client.GetWikiPageByID(context.Background(), "RND.wiki", 1578)
	if err != nil {
		t.Fatalf("GetWikiPageByID returned error: %v", err)
	}
	if page.ID != 1578 || page.Path != "/Source wallet check" || page.Content != "# Source wallet check\n\nRules." {
		t.Fatalf("unexpected page: %#v", page)
	}
}

func TestGetWikiPageByPathIncludesContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/RND/_apis/wiki/wikis/RND.wiki/pages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("path"); got != "/Guides/Source wallet" {
			t.Fatalf("unexpected page path: %q", got)
		}
		if got := r.URL.Query().Get("includeContent"); got != "true" {
			t.Fatalf("includeContent: got %q, want true", got)
		}
		fmt.Fprint(w, `{"path":"/Guides/Source wallet","content":"Page content"}`)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "RND", "test-pat", false, false, nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	page, err := client.GetWikiPageByPath(context.Background(), "RND.wiki", "/Guides/Source wallet")
	if err != nil {
		t.Fatalf("GetWikiPageByPath returned error: %v", err)
	}
	if page.Path != "/Guides/Source wallet" || page.Content != "Page content" {
		t.Fatalf("unexpected page: %#v", page)
	}
}
