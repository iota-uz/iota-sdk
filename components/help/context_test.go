package help

import (
	"bytes"
	"context"
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

// Falsely green if the assertions ran only with a Class set: five of the six
// Granite consumers pass none, and templ's class processor rejects a nil value.
func TestContext_ClassVariants(t *testing.T) {
	cases := map[string]any{
		"nil":         nil,
		"string":      "text-base font-normal",
		"templ class": templ.Classes("text-base", map[string]bool{"font-normal": true}),
	}
	for name, class := range cases {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			err := Context(ContextProps{Title: "Policy status", Class: class}).Render(context.Background(), &buf)
			require.NoError(t, err)

			html := buf.String()
			require.NotContains(t, html, "unknown-type")
			require.Contains(t, html, "relative inline-flex shrink-0")
			require.Equal(t, class != nil, strings.Contains(html, "font-normal"))
		})
	}
}

func TestContext_AccessibilityContract(t *testing.T) {
	var buf bytes.Buffer
	err := Context(ContextProps{Title: "Policy status", Label: "About this page"}).Render(context.Background(), &buf)
	require.NoError(t, err)

	html := buf.String()
	require.Equal(t, 1, strings.Count(html, `aria-haspopup="dialog"`))
	require.Contains(t, html, `aria-label="About this page"`)
	require.Contains(t, html, `role="dialog"`)
}

// Falsely green if only Context were covered: Hint takes the same Class prop.
func TestHint_ClassVariants(t *testing.T) {
	for name, class := range map[string]any{"nil": nil, "string": "ml-1"} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			err := Hint(HintProps{Title: "Deadline", Description: "Pick a date.", Class: class}).Render(context.Background(), &buf)
			require.NoError(t, err)
			require.NotContains(t, buf.String(), "unknown-type")
		})
	}
}
