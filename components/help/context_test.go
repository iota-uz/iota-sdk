package help

import (
	"bytes"
	"context"
	"html"
	"regexp"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"
)

func TestContext_Render(t *testing.T) {
	var buf bytes.Buffer
	err := Context(ContextProps{
		Title:        "Policy status",
		Summary:      "Check coverage before taking an action.",
		ArticlePath:  "insurance/policy-lifecycle.md#status",
		ArticleLabel: "Read guide",
		Sections: []ContextSection{{
			Title: "Check first",
			Items: []string{"Payment", "Coverage dates"},
		}},
	}).Render(context.Background(), &buf)
	require.NoError(t, err)

	html := buf.String()
	require.Contains(t, html, "Policy status")
	require.Contains(t, html, "Check first")
	require.Contains(t, html, "/help/doc/insurance/policy-lifecycle.md#status")
	require.Contains(t, html, `role="dialog"`)
}

func TestHint_Render(t *testing.T) {
	var buf bytes.Buffer
	err := Hint(HintProps{
		Title:        "Signing deadline",
		Description:  "The date by which every required signature must be collected.",
		Next:         "Choose a realistic date before sending the document.",
		ArticlePath:  "edo/create-document.md#deadline",
		ArticleLabel: "More details",
	}).Render(context.Background(), &buf)
	require.NoError(t, err)

	html := buf.String()
	require.Contains(t, html, "Signing deadline")
	require.Contains(t, html, "Choose a realistic date")
	require.Contains(t, html, "/help/doc/edo/create-document.md#deadline")
}

func renderToString(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, c.Render(context.Background(), &buf))
	return html.UnescapeString(buf.String())
}

func firstClassTokens(t *testing.T, markup string) []string {
	t.Helper()
	m := regexp.MustCompile(`class="([^"]*)"`).FindStringSubmatch(markup)
	require.NotNil(t, m)
	return strings.Fields(m[1])
}

var classCases = map[string]any{
	"nil":          nil,
	"string":       "ml-1",
	"templ class":  templ.Classes("ml-1"),
	"nested slice": []any{nil, "ml-1"},
	"conditional":  map[string]bool{"ml-1": true, "mr-1": false},
}

// Falsely green if the nil case were skipped: most consumers pass no Class, and
// templ's class processor rejects nil and []any with an "unknown-type" marker.
func assertClassVariants(t *testing.T, render func(class any) string) {
	t.Helper()
	baseline := firstClassTokens(t, render(nil))
	require.NotEmpty(t, baseline)
	for name, class := range classCases {
		t.Run(name, func(t *testing.T) {
			markup := render(class)
			require.NotContains(t, markup, "unknown-type")
			tokens := firstClassTokens(t, markup)
			require.Subset(t, tokens, baseline)
			if class != nil {
				require.Contains(t, tokens, "ml-1")
			}
			require.NotContains(t, tokens, "mr-1")
		})
	}
}

func TestContext_ClassVariants(t *testing.T) {
	assertClassVariants(t, func(class any) string {
		return renderToString(t, Context(ContextProps{Title: "Policy status", Class: class}))
	})
}

func TestHint_ClassVariants(t *testing.T) {
	assertClassVariants(t, func(class any) string {
		return renderToString(t, Hint(HintProps{Title: "Deadline", Description: "Pick a date.", Class: class}))
	})
}

func TestLink_ClassVariants(t *testing.T) {
	assertClassVariants(t, func(class any) string {
		return renderToString(t, Link(LinkProps{Path: "modules/crm.md", Class: class}))
	})
}

// Falsely green if only presence were checked: the button must point at the
// panel's own id and the panel must be labelled by the heading's own id, and
// both names must be declared in x-id so Alpine mints them per instance.
func TestContext_AccessibilityPairs(t *testing.T) {
	markup := renderToString(t, Context(ContextProps{Title: "Policy status", Label: "About this page"}))

	require.Equal(t, 1, strings.Count(markup, `aria-haspopup="dialog"`))
	require.Contains(t, markup, `aria-label="About this page"`)

	name := func(attr string) string {
		m := regexp.MustCompile(regexp.QuoteMeta(attr) + `="\$id\('([^']+)'\)"`).FindStringSubmatch(markup)
		require.NotNil(t, m, attr)
		return m[1]
	}
	dialogTag := regexp.MustCompile(`<div[^>]*role="dialog"[^>]*>`).FindString(markup)
	require.NotEmpty(t, dialogTag)
	idMatch := regexp.MustCompile(` :id="\$id\('([^']+)'\)"`).FindStringSubmatch(dialogTag)
	require.NotNil(t, idMatch)
	controls, panelID := name(":aria-controls"), idMatch[1]
	labelledBy, titleID := name(":aria-labelledby"), name("<h2 :id")
	require.Equal(t, controls, panelID)
	require.Equal(t, labelledBy, titleID)
	require.NotEqual(t, controls, labelledBy)

	declared := regexp.MustCompile(`x-id="\[([^\]]*)\]"`).FindStringSubmatch(markup)
	require.NotNil(t, declared)
	require.Contains(t, declared[1], "'"+controls+"'")
	require.Contains(t, declared[1], "'"+labelledBy+"'")
}

// Falsely green if the check only looked for a dialog anywhere in the output:
// consumers place Context inside headings, so the panel must not render inside
// the component's own element (it is teleported to the body).
func TestContext_PanelIsNotRenderedInsideTheRoot(t *testing.T) {
	markup := renderToString(t, Context(ContextProps{Title: "Policy status", Summary: "Check coverage."}))

	root, panel, found := strings.Cut(markup, `<template x-teleport="body">`)
	require.True(t, found)
	require.Contains(t, root, `aria-haspopup="dialog"`)
	require.NotContains(t, root, `role="dialog"`)
	require.NotContains(t, root, "<h2")
	require.Contains(t, panel, `role="dialog"`)
	require.Contains(t, panel, "<h2")
}

// Falsely green if only the dialog's presence were checked: keyboard users need
// a focusable container to land on when the panel opens (it is teleported to the
// end of the body), and a tall panel must be able to scroll internally.
func TestContext_PanelIsFocusableAndScrollable(t *testing.T) {
	markup := renderToString(t, Context(ContextProps{Title: "Policy status"}))

	_, panel, found := strings.Cut(markup, `<template x-teleport="body">`)
	require.True(t, found)
	open := regexp.MustCompile(`<div[^>]*role="dialog"[^>]*>`).FindString(panel)
	require.NotEmpty(t, open)
	require.Contains(t, open, `tabindex="-1"`)
	require.Contains(t, strings.Fields(regexp.MustCompile(`class="([^"]*)"`).FindStringSubmatch(open)[1]), "overflow-y-auto")
}
