package cli

import (
	"bytes"
	"strings"
	"testing"

	"tfs-cli/internal/api"
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
