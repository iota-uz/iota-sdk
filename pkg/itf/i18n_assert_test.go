package itf

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func testBundle(tb testing.TB) *i18n.Bundle {
	tb.Helper()
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	// Covers both nesting shapes the hint talks about: a table [Reserve.Export]
	// with keys, and a deeper table [Reserve.Export.Reason].
	const en = `
[Reserve]
[Reserve.Export]
Title = "Reserve export"
[Reserve.Export.Reason]
Status = "Excluded by status"
`
	_, err := bundle.ParseMessageFileBytes([]byte(en), "en.toml")
	require.NoError(tb, err)
	const ru = `
[Reserve]
[Reserve.Export]
Title = "Экспорт резервов"
`
	_, err = bundle.ParseMessageFileBytes([]byte(ru), "ru.toml")
	require.NoError(tb, err)
	return bundle
}

func TestRequireMessage_ResolvesNestedID(t *testing.T) {
	t.Parallel()

	bundle := testBundle(t)
	msg := RequireMessage(t, bundle, "en", "Reserve.Export.Title")
	assert.Equal(t, "Reserve export", msg)
}

func TestRequireMessage_ResolvesDeeplyNestedID(t *testing.T) {
	t.Parallel()

	bundle := testBundle(t)
	// The dot-path trap: the id must resolve through nested tables; a flat
	// key spelling out the same dots does not match.
	msg := RequireMessage(t, bundle, "en", "Reserve.Export.Reason.Status")
	assert.Equal(t, "Excluded by status", msg)
}

func TestMessageResolves(t *testing.T) {
	t.Parallel()

	bundle := testBundle(t)

	assert.True(t, MessageResolves(bundle, "en", "Reserve.Export.Title"))
	assert.True(t, MessageResolves(bundle, "en", "Reserve.Export.Reason.Status"))

	// Unknown id: no such path in any locale.
	assert.False(t, MessageResolves(bundle, "en", "Reserve.Export.Missing"))
	// The dot-path trap: the flat spelling does not resolve.
	assert.False(t, MessageResolves(bundle, "en", "Reserve.Export.Title.Status"))
	// Key shipped in en but missing in ru must NOT count as resolving for ru,
	// even though the localizer can fall back to the en string.
	assert.False(t, MessageResolves(bundle, "ru", "Reserve.Export.Reason.Status"))
	// Key present in both locales resolves for both.
	assert.True(t, MessageResolves(bundle, "ru", "Reserve.Export.Title"))
}

func TestRequireMessageAllLocales(t *testing.T) {
	t.Parallel()

	bundle := testBundle(t)

	// Present everywhere passes.
	RequireMessageAllLocales(t, bundle, "Reserve.Export.Title", "en", "ru")

	// Partial translation is caught through the predicate rather than by
	// failing a subtest (which would mark the parent red regardless).
	assert.False(t, MessageResolves(bundle, "ru", "Reserve.Export.Reason.Status"))
}

func TestRequireMessages_ChecksEveryID(t *testing.T) {
	t.Parallel()

	bundle := testBundle(t)

	RequireMessages(t, bundle, "en", "Reserve.Export.Title", "Reserve.Export.Reason.Status")

	assert.False(t, MessageResolves(bundle, "en", "Reserve.Export.Nothing"))
}
