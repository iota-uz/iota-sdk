package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithSupportedLanguagesCopiesInput(t *testing.T) {
	languages := []string{"en", "ru", "uz", "uz-Cyrl"}
	opts := &options{}

	WithSupportedLanguages(languages)(opts)
	languages[0] = "changed"

	require.Equal(t, []string{"en", "ru", "uz", "uz-Cyrl"}, opts.supportedLanguages)
}
