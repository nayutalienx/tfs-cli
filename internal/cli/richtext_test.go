package cli

import (
	"strings"
	"testing"
)

func TestRenderRichTextConvertsMarkdownStructure(t *testing.T) {
	input := "### План\n\nТекст с `кодом`.\n\n- Первый пункт\n- Второй пункт"

	got, err := renderRichText(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, fragment := range []string{
		"<h3>План</h3>",
		"<p>Текст с <code>кодом</code>.</p>",
		"<ul>",
		"<li>Первый пункт</li>",
		"<li>Второй пункт</li>",
	} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("rendered HTML does not contain %q:\n%s", fragment, got)
		}
	}
}

func TestRenderRichTextPreservesExistingHTML(t *testing.T) {
	input := "<h3>План</h3><p><strong>Готово</strong></p>"

	got, err := renderRichText(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != input {
		t.Fatalf("existing HTML changed:\n got: %s\nwant: %s", got, input)
	}
}

func TestBuildPatchRendersAllKnownRichTextFields(t *testing.T) {
	sets := []string{
		"System.Title=Обычный заголовок",
		"System.Description=### Описание\n\nТекст",
		"Microsoft.VSTS.Common.AcceptanceCriteria=- Критерий один\n- Критерий два",
		"Microsoft.VSTS.TCM.ReproSteps=1. Первый шаг\n2. Второй шаг",
	}

	patch, err := buildPatch(sets, "### Комментарий\n\n- Пункт")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	values := map[string]string{}
	for _, operation := range patch {
		path, _ := operation["path"].(string)
		value, _ := operation["value"].(string)
		values[path] = value
	}

	if got := values["/fields/System.Title"]; got != "Обычный заголовок" {
		t.Fatalf("plain-text field changed: %s", got)
	}
	assertContainsAll(t, values["/fields/System.Description"], "<h3>Описание</h3>", "<p>Текст</p>")
	assertContainsAll(t, values["/fields/Microsoft.VSTS.Common.AcceptanceCriteria"], "<ul>", "<li>Критерий один</li>")
	assertContainsAll(t, values["/fields/Microsoft.VSTS.TCM.ReproSteps"], "<ol>", "<li>Первый шаг</li>")
	assertContainsAll(t, values["/fields/System.History"], "<h3>Комментарий</h3>", "<li>Пункт</li>")
}

func assertContainsAll(t *testing.T, value string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(value, fragment) {
			t.Fatalf("value does not contain %q:\n%s", fragment, value)
		}
	}
}
