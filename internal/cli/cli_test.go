package cli

import (
	"strings"
	"testing"
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
