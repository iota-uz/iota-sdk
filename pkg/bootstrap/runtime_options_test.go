package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithSupportedLanguagesCopiesInput(t *testing.T) {
	languages := []string{"en", "ru", "uz", "uz-Cyrl"}
	opts := &options{}

	option := WithSupportedLanguages(languages)
	languages[0] = "changed"
	option(opts)

	require.Equal(t, []string{"en", "ru", "uz", "uz-Cyrl"}, opts.supportedLanguages)
}
