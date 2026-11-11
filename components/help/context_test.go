package help

import (
	"bytes"
	"context"
	"testing"

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

func TestContext_TriggerStyleAndPanelPlacement(t *testing.T) {
	var buf bytes.Buffer
	err := Context(ContextProps{Title: "Policy status"}).Render(context.Background(), &buf)
	require.NoError(t, err)

	html := buf.String()
	require.Contains(t, html, "rounded-full")
	require.Contains(t, html, "text-200")
	require.Contains(t, html, "aria-expanded:text-brand-500")
	require.NotContains(t, html, "border-default bg-surface-300 text-gray-700 shadow-sm")
	require.Contains(t, html, "calc(100vw-2rem)")
	require.Contains(t, html, `x-ref="trigger"`)
	require.Contains(t, html, `x-ref="panel"`)
	require.Contains(t, html, "document.documentElement.clientWidth")
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
