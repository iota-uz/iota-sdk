package dialog

import (
	"bytes"
	"context"
	"testing"

	"github.com/PuerkitoBio/goquery"
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
