package cli

import (
	"bytes"
	"strings"
	"testing"

	"tfs-cli/internal/api"
	"tfs-cli/internal/output"
)

func TestParseAssignment(t *testing.T) {
	field, value, err := parseAssignment("System.Title=Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field != "System.Title" {
		t.Fatalf("unexpected field: %s", field)
	}
	if value != "Hello" {
		t.Fatalf("unexpected value: %s", value)
	}
}

func TestParseAssignmentInvalid(t *testing.T) {
	_, _, err := parseAssignment("System.Title")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestWiqlSearchQueryEscapesQuotes(t *testing.T) {
	q := wiqlSearchQuery("it's")
	expected := "SELECT [System.Id] FROM WorkItems WHERE ([System.Title] CONTAINS 'it''s' OR [System.Description] CONTAINS 'it''s') ORDER BY [System.ChangedDate] DESC"
	if q != expected {
		t.Fatalf("unexpected query: %s", q)
	}
}

func TestMyWiqlQuery(t *testing.T) {
	expected := "SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = @Project AND [System.AssignedTo] = @Me AND [System.State] IN ('Разработка', 'Выполняется') ORDER BY [System.ChangedDate] DESC"
	got := myWiqlQuery("", true, "", false)
	if got != expected {
		t.Fatalf("unexpected query: %s", got)
	}
}

func TestMyWiqlQueryExcludeStateOverridesDefault(t *testing.T) {
	expected := "SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = @Project AND [System.AssignedTo] = @Me AND [System.State] <> 'Выполнено' ORDER BY [System.ChangedDate] DESC"
	got := myWiqlQuery("", true, "Выполнено", false)
	if got != expected {
		t.Fatalf("unexpected query: %s", got)
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	base, ok := normalizeBaseURL("https://tfs.example.com/DefaultCollection/MyProject", "MyProject")
	if !ok {
		t.Fatalf("expected normalization")
	}
	if base != "https://tfs.example.com/DefaultCollection" {
		t.Fatalf("unexpected base: %s", base)
	}
}

func TestSplitPositional(t *testing.T) {
	args := []string{"123", "--set", "System.Title=Hello", "--add-comment", "note", "--verbose"}
	positional, rest := splitPositional(args, updateValueFlags())
	if positional != "123" {
		t.Fatalf("expected positional 123, got %s", positional)
	}
	joined := strings.Join(rest, " ")
	if !strings.Contains(joined, "--set") || !strings.Contains(joined, "System.Title=Hello") {
		t.Fatalf("flags missing from rest: %s", joined)
	}
}

func TestSplitPositionalDeleteFlags(t *testing.T) {
	args := []string{"--destroy", "123", "--yes"}
	positional, rest := splitPositional(args, deleteValueFlags())
	if positional != "123" {
		t.Fatalf("expected positional 123, got %s", positional)
	}
	joined := strings.Join(rest, " ")
	if !strings.Contains(joined, "--destroy") || !strings.Contains(joined, "--yes") {
		t.Fatalf("flags missing from rest: %s", joined)
	}
}

func TestDeleteRequiresConfirmation(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"delete", "123"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "confirmation_required") {
		t.Fatalf("expected confirmation error, got %s", stderr.String())
	}
}

func TestNormalizeGitRef(t *testing.T) {
	tests := map[string]string{
		"feature/test":            "refs/heads/feature/test",
		"heads/feature/test":      "refs/heads/feature/test",
		"refs/heads/feature/test": "refs/heads/feature/test",
	}

	for input, expected := range tests {
		if got := normalizeGitRef(input); got != expected {
			t.Fatalf("normalizeGitRef(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestPullRequestURLPrefersWebLink(t *testing.T) {
	pr := api.GitPullRequest{
		PullRequestID: 42,
		URL:           "https://tfs.example/_apis/git/repositories/repo/pullrequests/42",
		Repository: api.GitRepository{
			RemoteURL: "https://tfs.example/DefaultCollection/Proj/_git/repo",
		},
		Links: map[string]api.Link{
			"web": {Href: "https://tfs.example/DefaultCollection/Proj/_git/repo/pullrequest/42"},
		},
	}

	if got := pullRequestURL(pr); got != "https://tfs.example/DefaultCollection/Proj/_git/repo/pullrequest/42" {
		t.Fatalf("unexpected PR URL: %s", got)
	}
}

func TestParsePositiveIDs(t *testing.T) {
	ids, err := parsePositiveIDs([]string{"12", "34"}, "work item")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != 12 || ids[1] != 34 {
		t.Fatalf("unexpected ids: %#v", ids)
	}
}

func TestParsePositiveIDsRejectsInvalidValue(t *testing.T) {
	_, err := parsePositiveIDs([]string{"abc"}, "work item")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "work item id must be a number") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResourceRefIDs(t *testing.T) {
	ids := resourceRefIDs([]api.ResourceRef{
		{ID: "101"},
		{ID: ""},
		{ID: "202"},
	})
	if strings.Join(ids, ",") != "101,202" {
		t.Fatalf("unexpected resource ref ids: %#v", ids)
	}
}

func TestParsePullRequestURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantBase string
		wantProj string
		wantRepo string
		wantID   int
	}{
		{
			name:     "on-prem with collection and query",
			input:    "https://tfs.example.com/DefaultCollection/Project/_git/repo-name/pullrequest/37456?_a=overview",
			wantBase: "https://tfs.example.com/DefaultCollection",
			wantProj: "Project",
			wantRepo: "repo-name",
			wantID:   37456,
		},
		{
			name:     "on-prem no query",
			input:    "https://tfs.example.com/DefaultCollection/Project/_git/repo-name/pullrequest/37456",
			wantBase: "https://tfs.example.com/DefaultCollection",
			wantProj: "Project",
			wantRepo: "repo-name",
			wantID:   37456,
		},
		{
			name:     "azure devops services",
			input:    "https://dev.azure.com/org/project/_git/repo/pullrequest/42",
			wantBase: "https://dev.azure.com/org",
			wantProj: "project",
			wantRepo: "repo",
			wantID:   42,
		},
		{
			name:     "plural pullrequests segment",
			input:    "https://tfs.example.com/DefaultCollection/Project/_git/repo-name/pullrequests/99",
			wantBase: "https://tfs.example.com/DefaultCollection",
			wantProj: "Project",
			wantRepo: "repo-name",
			wantID:   99,
		},
		{
			name:     "no collection in path",
			input:    "https://tfs.example.com/Project/_git/repo/pullrequest/5",
			wantBase: "https://tfs.example.com",
			wantProj: "Project",
			wantRepo: "repo",
			wantID:   5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			loc, err := parsePullRequestURL(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if loc.BaseURL != tc.wantBase {
				t.Fatalf("baseURL: got %q, want %q", loc.BaseURL, tc.wantBase)
			}
			if loc.Project != tc.wantProj {
				t.Fatalf("project: got %q, want %q", loc.Project, tc.wantProj)
			}
			if loc.Repository != tc.wantRepo {
				t.Fatalf("repository: got %q, want %q", loc.Repository, tc.wantRepo)
			}
			if loc.PullRequestID != tc.wantID {
				t.Fatalf("prID: got %d, want %d", loc.PullRequestID, tc.wantID)
			}
		})
	}
}

func TestParsePullRequestURLInvalid(t *testing.T) {
	tests := []string{
		"not-a-url",
		"https://tfs.example.com/DefaultCollection/Project/_git/repo-name",
		"https://tfs.example.com/DefaultCollection/Project/_git/repo-name/pullrequest/",
		"https://tfs.example.com/DefaultCollection/Project/_git/repo-name/pullrequest/abc",
		"https://tfs.example.com/_git/repo/pullrequest/1",
	}
	for _, input := range tests {
		_, err := parsePullRequestURL(input)
		if err == nil {
			t.Fatalf("expected error for %q", input)
		}
	}
}

func TestParseWikiPageURLByID(t *testing.T) {
	rawURL := "https://tfs.solarlab.ru/DefaultCollection/RND/_wiki/wikis/RND.wiki/1578/%D0%9F%D1%80%D0%BE%D0%B2%D0%B5%D1%80%D0%BA%D0%B0-%D0%BA%D0%BE%D1%88%D0%B5%D0%BB%D1%8C%D0%BA%D0%B0"
	locator, err := parseWikiPageURL(rawURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if locator.BaseURL != "https://tfs.solarlab.ru/DefaultCollection" {
		t.Fatalf("unexpected base URL: %s", locator.BaseURL)
	}
	if locator.Project != "RND" || locator.WikiIdentifier != "RND.wiki" || locator.PageID != 1578 {
		t.Fatalf("unexpected locator: %#v", locator)
	}
	if locator.PagePath != "" || locator.SourceURL != rawURL {
		t.Fatalf("unexpected locator source fields: %#v", locator)
	}
}

func TestParseWikiPageURLByPath(t *testing.T) {
	rawURL := "https://dev.azure.com/example-org/RND/_wiki/wikis/Architecture.wiki?pagePath=%2FGuides%2FSource+wallet"
	locator, err := parseWikiPageURL(rawURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if locator.BaseURL != "https://dev.azure.com/example-org" || locator.Project != "RND" {
		t.Fatalf("unexpected collection locator: %#v", locator)
	}
	if locator.WikiIdentifier != "Architecture.wiki" || locator.PageID != 0 || locator.PagePath != "/Guides/Source wallet" {
		t.Fatalf("unexpected page locator: %#v", locator)
	}
}

func TestParseWikiPageURLInvalid(t *testing.T) {
	tests := []string{
		"not-a-url",
		"https://tfs.example/DefaultCollection/RND/_wiki/wikis/RND.wiki",
		"https://tfs.example/DefaultCollection/RND/_wiki/wikis/RND.wiki/not-a-page-id/Page",
		"https://tfs.example/DefaultCollection/RND/_wiki/wikis/RND.wiki/0/Page",
		"https://tfs.example/DefaultCollection/RND/_workitems/edit/1578",
		"https://user@tfs.example/DefaultCollection/RND/_wiki/wikis/RND.wiki/1578/Page",
	}
	for _, input := range tests {
		if _, err := parseWikiPageURL(input); err == nil {
			t.Fatalf("expected error for %q", input)
		}
	}
}

func TestSameTFSBaseURL(t *testing.T) {
	if !sameTFSBaseURL("https://TFS.example/DefaultCollection/", "https://tfs.example/defaultcollection") {
		t.Fatal("expected matching TFS base URLs")
	}
	if sameTFSBaseURL("https://tfs.example/DefaultCollection", "https://evil.example/DefaultCollection") {
		t.Fatal("expected different hosts to be rejected")
	}
	if sameTFSBaseURL("https://tfs.example/DefaultCollection", "https://tfs.example/OtherCollection") {
		t.Fatal("expected different collections to be rejected")
	}
}

func TestRenderWikiPageText(t *testing.T) {
	var stdout bytes.Buffer
	ctx := commandContext{stdout: &stdout, stderr: &bytes.Buffer{}}
	locator := wikiPageLocator{WikiIdentifier: "RND.wiki", PageID: 1578, SourceURL: "https://tfs.example/wiki-page"}
	page := api.WikiPage{
		Path:        "/Source wallet check",
		GitItemPath: "/Source-wallet-check.md",
		Content:     "# Source wallet check\n\nRules.",
	}

	if code := renderWikiPage(ctx, locator, page); code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	text := stdout.String()
	for _, expected := range []string{"Wiki: RND.wiki", "PageID: 1578", "Path: /Source wallet check", "GitItemPath: /Source-wallet-check.md", "Content:\n# Source wallet check\n\nRules."} {
		if !strings.Contains(text, expected) {
			t.Fatalf("rendered output missing %q:\n%s", expected, text)
		}
	}
}

func TestRenderWikiPageJSONIncludesContentOnce(t *testing.T) {
	var stdout bytes.Buffer
	ctx := commandContext{jsonMode: true, stdout: &stdout, stderr: &bytes.Buffer{}}
	locator := wikiPageLocator{WikiIdentifier: "RND.wiki", PageID: 1578, SourceURL: "https://tfs.example/wiki-page"}
	page := api.WikiPage{ID: 1578, Path: "/Page", Content: "unique document body"}

	if code := renderWikiPage(ctx, locator, page); code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	text := stdout.String()
	if count := strings.Count(text, "unique document body"); count != 1 {
		t.Fatalf("content occurred %d times, want exactly once: %s", count, text)
	}
	if strings.Contains(text, `"raw"`) {
		t.Fatalf("JSON output should not duplicate the page in a raw field: %s", text)
	}
}

func TestIsURL(t *testing.T) {
	if !isURL("https://example.com") {
		t.Fatalf("expected https URL to be detected")
	}
	if !isURL("http://example.com") {
		t.Fatalf("expected http URL to be detected")
	}
	if isURL("37456") {
		t.Fatalf("expected bare id not to be detected as URL")
	}
	if isURL("--flag") {
		t.Fatalf("expected flag not to be detected as URL")
	}
}

func TestShortRef(t *testing.T) {
	if got := shortRef("refs/heads/feature/xyz"); got != "feature/xyz" {
		t.Fatalf("unexpected short ref: %s", got)
	}
	if got := shortRef("refs/heads/develop"); got != "develop" {
		t.Fatalf("unexpected short ref: %s", got)
	}
	if got := shortRef("refs/tags/v1.0"); got != "refs/tags/v1.0" {
		t.Fatalf("unexpected short ref: %s", got)
	}
	if got := shortRef(""); got != "" {
		t.Fatalf("unexpected short ref: %s", got)
	}
}

func TestIdentityDisplayName(t *testing.T) {
	if got := identityDisplayName(map[string]interface{}{"displayName": "John Doe", "uniqueName": "john@example.com"}); got != "John Doe" {
		t.Fatalf("unexpected display name: %s", got)
	}
	if got := identityDisplayName(map[string]interface{}{"uniqueName": "john@example.com"}); got != "john@example.com" {
		t.Fatalf("unexpected display name: %s", got)
	}
	if got := identityDisplayName(nil); got != "" {
		t.Fatalf("unexpected display name: %s", got)
	}
	if got := identityDisplayName(map[string]interface{}{"name": "Legacy User <legacy@example.com>"}); got != "Legacy User <legacy@example.com>" {
		t.Fatalf("unexpected legacy name: %s", got)
	}
}

func TestPrintWorkItemDetailsIncludesAllComments(t *testing.T) {
	title := "Example item"
	wiType := "Feature"
	state := "Active"
	wi := output.WorkItem{
		ID:    42,
		Title: &title,
		Type:  &wiType,
		State: &state,
	}
	comments := []api.WorkItemComment{
		{
			Revision:    2,
			Text:        "first line\nsecond line",
			RevisedBy:   map[string]interface{}{"displayName": "First User"},
			RevisedDate: "2026-07-20T10:00:00Z",
		},
		{
			Revision:  5,
			Text:      "latest comment",
			RevisedBy: map[string]interface{}{"name": "Legacy User"},
		},
	}

	var rendered bytes.Buffer
	printWorkItemDetails(&rendered, wi, map[string]interface{}{
		"System.Description": "Description text",
		"System.History":     "latest comment",
	}, comments, nil)

	text := rendered.String()
	for _, expected := range []string{
		"Description text",
		"Comments:",
		"revision 2 | First User | 2026-07-20T10:00:00Z",
		"first line",
		"second line",
		"revision 5 | Legacy User",
		"latest comment",
		"Children: none",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("rendered output missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "Comment (latest):") {
		t.Fatalf("latest comment should not be printed separately:\n%s", text)
	}
}
