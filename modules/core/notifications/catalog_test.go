package notifications

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCatalogRegistration(t *testing.T) {
	c := NewCatalog()
	require.NoError(t, c.Register(TestDefinition()))
	require.Error(t, c.Register(TestDefinition()))
	require.Error(t, c.Register(Definition{Key: "missing.renderer", Name: map[string]string{"en": "Invalid"}}))
	_, ok := c.Get("unknown")
	require.False(t, ok)
	require.Len(t, c.Definitions(), 1)
}
func TestDefinitionRendersRecipientLanguage(t *testing.T) {
	for _, locale := range []string{"en", "ru", "uz", "uz-Cyrl", "pt-BR", "zh"} {
		t.Run(locale, func(t *testing.T) {
			d := TestDefinition()
			content, err := d.Render(Event{}, locale)
			require.NoError(t, err)
			require.Equal(t, d.Name[locale], content.Title)
			require.NotEmpty(t, content.Body)
		})
	}
	content, err := TestDefinition().Render(Event{}, "unsupported")
	require.NoError(t, err)
	require.Equal(t, "Test notification", content.Title)
}
