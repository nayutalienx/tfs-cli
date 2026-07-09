package diff

import (
	"strings"
	"testing"
)

func TestUnifiedDiff_Add(t *testing.T) {
	oldText := ""
	newText := "line1\nline2\nline3\n"
	got := UnifiedDiff(oldText, newText)
	if got == "" {
		t.Fatal("expected non-empty diff for added content")
	}
	if !strings.Contains(got, "+line1") || !strings.Contains(got, "+line2") || !strings.Contains(got, "+line3") {
		t.Errorf("diff should contain all added lines, got:\n%s", got)
	}
	if strings.Contains(got, "-line") {
		t.Errorf("diff should not contain deleted lines, got:\n%s", got)
	}
}

func TestUnifiedDiff_Delete(t *testing.T) {
	oldText := "line1\nline2\nline3\n"
	newText := ""
	got := UnifiedDiff(oldText, newText)
	if got == "" {
		t.Fatal("expected non-empty diff for deleted content")
	}
	if !strings.Contains(got, "-line1") || !strings.Contains(got, "-line2") || !strings.Contains(got, "-line3") {
		t.Errorf("diff should contain all deleted lines, got:\n%s", got)
	}
	if strings.Contains(got, "+line") {
		t.Errorf("diff should not contain inserted lines, got:\n%s", got)
	}
}

func TestUnifiedDiff_Edit(t *testing.T) {
	oldText := "line1\nold2\nline3\n"
	newText := "line1\nnew2\nline3\n"
	got := UnifiedDiff(oldText, newText)
	if got == "" {
		t.Fatal("expected non-empty diff for edited content")
	}
	if !strings.Contains(got, "-old2") {
		t.Errorf("diff should contain deleted old line, got:\n%s", got)
	}
	if !strings.Contains(got, "+new2") {
		t.Errorf("diff should contain inserted new line, got:\n%s", got)
	}
	if !strings.Contains(got, " line1") {
		t.Errorf("diff should contain context line1, got:\n%s", got)
	}
	if !strings.Contains(got, " line3") {
		t.Errorf("diff should contain context line3, got:\n%s", got)
	}
}

func TestUnifiedDiff_NoChanges(t *testing.T) {
	oldText := "line1\nline2\nline3\n"
	newText := "line1\nline2\nline3\n"
	got := UnifiedDiff(oldText, newText)
	if got != "" {
		t.Errorf("expected empty diff for identical content, got:\n%s", got)
	}
}

func TestUnifiedDiff_BothEmpty(t *testing.T) {
	got := UnifiedDiff("", "")
	if got != "" {
		t.Errorf("expected empty diff for empty content, got:\n%s", got)
	}
}

func TestUnifiedDiff_HunkHeader(t *testing.T) {
	oldText := "line1\nline2\nline3\n"
	newText := "line1\nchanged\nline3\n"
	got := UnifiedDiff(oldText, newText)
	if !strings.Contains(got, "@@ -1,3 +1,3 @@") {
		t.Errorf("diff should contain correct hunk header, got:\n%s", got)
	}
}

func TestUnifiedDiff_InsertInMiddle(t *testing.T) {
	oldText := "a\nb\nc\nd\ne\n"
	newText := "a\nb\nX\nc\nd\ne\n"
	got := UnifiedDiff(oldText, newText)
	if !strings.Contains(got, "+X") {
		t.Errorf("diff should contain inserted line X, got:\n%s", got)
	}
	if !strings.Contains(got, " b") {
		t.Errorf("diff should contain context line b, got:\n%s", got)
	}
	if !strings.Contains(got, " c") {
		t.Errorf("diff should contain context line c, got:\n%s", got)
	}
}

func TestUnifiedDiff_MultipleHunks(t *testing.T) {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line" + string(rune('a'+i))
	}
	oldText := strings.Join(lines, "\n") + "\n"

	newLines := make([]string, 20)
	copy(newLines, lines)
	newLines[2] = "CHANGED1"
	newLines[15] = "CHANGED2"
	newText := strings.Join(newLines, "\n") + "\n"

	got := UnifiedDiff(oldText, newText)
	hunkCount := strings.Count(got, "@@")
	if hunkCount < 2 {
		t.Errorf("expected at least 2 hunks for separated changes, got %d:\n%s", hunkCount, got)
	}
}
