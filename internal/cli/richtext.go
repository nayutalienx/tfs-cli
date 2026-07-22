package cli

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"

	"tfs-cli/internal/errs"
)

var richTextMarkdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(goldmarkhtml.WithUnsafe()),
)

var richTextFields = map[string]struct{}{
	"system.description":                       {},
	"system.history":                           {},
	"microsoft.vsts.common.acceptancecriteria": {},
	"microsoft.vsts.tcm.reprosteps":            {},
}

func normalizeWorkItemFieldValue(field, value string) (string, error) {
	if _, ok := richTextFields[strings.ToLower(strings.TrimSpace(field))]; !ok {
		return value, nil
	}
	return renderRichText(value)
}

func renderRichText(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	var rendered bytes.Buffer
	if err := richTextMarkdown.Convert([]byte(value), &rendered); err != nil {
		return "", errs.New("rich_text_conversion_failed", "failed to convert Markdown to TFS rich text", err.Error())
	}
	return strings.TrimSpace(rendered.String()), nil
}
