package usercomponents

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLanguageSelect_KeepsSavedLanguage(t *testing.T) {
	t.Parallel()

	enabled := intl.GetSupportedLanguages([]string{"ru"})

	render := func(t *testing.T, value string) string {
		t.Helper()
		var buf bytes.Buffer
		err := LanguageSelect(&LanguageSelectProps{Value: value, Languages: enabled, Attrs: map[string]any{"name": "Language"}}).
			Render(context.Background(), &buf)
		require.NoError(t, err)
		return buf.String()
	}

	t.Run("saved language that is no longer enabled stays selectable", func(t *testing.T) {
		t.Parallel()
		html := render(t, "en")
		assert.Contains(t, html, `value="en" selected`)
		assert.Contains(t, html, `value="ru"`)
	})

	t.Run("enabled saved language is not duplicated", func(t *testing.T) {
		t.Parallel()
		html := render(t, "ru")
		assert.Equal(t, 1, strings.Count(html, `value="ru"`))
	})

	t.Run("unknown saved language adds nothing", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, enabled, withSavedLanguage(enabled, "xx"))
	})
}
