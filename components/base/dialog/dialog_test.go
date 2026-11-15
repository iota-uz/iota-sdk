package dialog

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfirmation_ControlsDoNotSubmitParentForm(t *testing.T) {
	var body bytes.Buffer
	body.WriteString(`<form id="save-form"><input required>`)
	require.NoError(t, Confirmation(&Props{Action: "confirm-delete"}).Render(context.Background(), &body))
	body.WriteString(`</form>`)
	doc, err := goquery.NewDocumentFromReader(&body)
	require.NoError(t, err)
	assert.Equal(t, 1, doc.Find("form").Length())
	buttons := doc.Find("#save-form dialog button")
	require.Equal(t, 3, buttons.Length())
	buttons.Each(func(_ int, button *goquery.Selection) {
		assert.Equal(t, "button", button.AttrOr("type", ""))
		assert.Equal(t, "$el.closest('dialog').close($el.value)", button.AttrOr("@click", ""))
	})
	assert.Equal(t, "cancel", buttons.First().AttrOr("value", ""))
	assert.Equal(t, "cancel", doc.Find(`[data-test-id="dialog-cancel-btn"]`).AttrOr("value", ""))
	assert.Equal(t, "confirm", doc.Find(`[data-test-id="dialog-confirm-btn"]`).AttrOr("value", ""))
}

func renderDoc(t *testing.T, render func(*bytes.Buffer) error) *goquery.Document {
	t.Helper()
	var body bytes.Buffer
	require.NoError(t, render(&body))
	doc, err := goquery.NewDocumentFromReader(&body)
	require.NoError(t, err)
	return doc
}

func TestStdViewDrawer_HeaderHasTitleAndAccessibleClose(t *testing.T) {
	doc := renderDoc(t, func(b *bytes.Buffer) error {
		return StdViewDrawer(StdDrawerProps{Title: "Edit group", Action: "edit"}).Render(context.Background(), b)
	})

	title := doc.Find("[data-dialog-title]")
	require.Equal(t, 1, title.Length())
	assert.Equal(t, "Edit group", strings.TrimSpace(title.Text()))
	assert.Equal(t, "-1", title.AttrOr("tabindex", ""))

	closeBtn := doc.Find(`[data-test-id="drawer-close"]`)
	require.Equal(t, 1, closeBtn.Length())
	assert.Contains(t, closeBtn.AttrOr("class", ""), "drawer-close")
	assert.Equal(t, "Close", closeBtn.AttrOr("aria-label", ""))
}

func TestDrawerFooter_CancelAndSaveWithoutDelete(t *testing.T) {
	doc := renderDoc(t, func(b *bytes.Buffer) error {
		return DrawerFooter(DrawerFooterProps{}).Render(context.Background(), b)
	})

	footer := doc.Find(`[data-test-id="drawer-footer"]`)
	require.Equal(t, 1, footer.Length())
	assert.Contains(t, footer.AttrOr("class", ""), "sticky bottom-0")
	assert.Contains(t, footer.AttrOr("class", ""), "bg-surface-300")
	assert.Equal(t, "Cancel", strings.TrimSpace(doc.Find(`[data-test-id="drawer-cancel"]`).Text()))
	save := doc.Find(`[data-test-id="drawer-save"]`)
	assert.Equal(t, "submit", save.AttrOr("type", ""))
	assert.Equal(t, "Save", strings.TrimSpace(save.Text()))
	assert.Equal(t, 0, doc.Find(`[data-test-id="drawer-delete"]`).Length())
}

func TestDrawerFooter_DeleteNeedsConfirmation(t *testing.T) {
	doc := renderDoc(t, func(b *bytes.Buffer) error {
		return DrawerFooter(DrawerFooterProps{
			Delete: &DrawerDelete{Attrs: templ.Attributes{"hx-delete": "/items/1"}},
		}).Render(context.Background(), b)
	})

	del := doc.Find(`[data-test-id="drawer-delete"]`)
	require.Equal(t, 1, del.Length())
	assert.Equal(t, "button", del.AttrOr("type", ""))
	assert.Equal(t, "confirmingDelete = true", del.AttrOr("@click", ""))
	assert.Empty(t, del.AttrOr("hx-delete", ""), "the first click must not reach the server")

	confirm := doc.Find(`[data-test-id="drawer-delete-confirm"]`)
	require.Equal(t, 1, confirm.Length())
	assert.Equal(t, "/items/1", confirm.AttrOr("hx-delete", ""))
	assert.Equal(t, "button", confirm.AttrOr("type", ""))
}

func TestConfirmation_DangerRendersDestructiveConfirm(t *testing.T) {
	doc := renderDoc(t, func(b *bytes.Buffer) error {
		return Confirmation(&Props{Action: "confirm-delete", Danger: true}).Render(context.Background(), b)
	})
	assert.Contains(t, doc.Find(`[data-test-id="dialog-confirm-btn"]`).AttrOr("class", ""), "btn-danger")
}
