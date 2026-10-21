package notifications

import (
	"testing"

	"github.com/google/uuid"

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

func TestCatalogEnvelopeValidation(t *testing.T) {
	c := NewCatalog()
	d := TestDefinition()
	d.PayloadFields = []PayloadField{{Key: "name", Required: true}}
	require.NoError(t, c.Register(d))
	event := Event{Key: TestEventKey, ID: "identity", TenantID: uuid.New(), Data: map[string]string{"name": "Alice"}}
	normalized, err := c.Normalize(event)
	require.NoError(t, err)
	require.Equal(t, 1, normalized.Version)
	require.False(t, normalized.OccurredAt.IsZero())
	event.Version = 2
	_, err = c.Normalize(event)
	require.Error(t, err)
	event.Version = 1
	event.Data = nil
	_, err = c.Normalize(event)
	require.Error(t, err)
	event.Data = map[string]string{"name": "Alice"}
	event.Subject = SubjectReference{Type: "document"}
	_, err = c.Normalize(event)
	require.Error(t, err)
	require.Len(t, c.ByModule("core"), 1)
	require.Empty(t, c.ByModule("crm"))
}
func TestCatalogRejectsInvalidMetadata(t *testing.T) {
	for _, key := range []string{"unversioned", "core.test.v0", "core.test.v-1"} {
		d := TestDefinition()
		d.Key = key
		require.Error(t, NewCatalog().Register(d))
	}
	d := TestDefinition()
	d.DefaultLevel = "critical"
	require.Error(t, NewCatalog().Register(d))
	d = TestDefinition()
	d.DefaultRecipientKeys = []string{"creator"}
	require.Error(t, NewCatalog().Register(d))
}

func TestCatalogMetadataCannotBeMutatedAfterRegistration(t *testing.T) {
	c := NewCatalog()
	d := TestDefinition()
	require.NoError(t, c.Register(d))
	d.Name["en"] = "Changed"
	got, ok := c.Get(d.Key)
	require.True(t, ok)
	require.Equal(t, "Test notification", got.Name["en"])
	got.Name["en"] = "Changed again"
	got, _ = c.Get(d.Key)
	require.Equal(t, "Test notification", got.Name["en"])
}
