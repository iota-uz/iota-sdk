package intl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSupportedLanguagesIncludesUzCyrl(t *testing.T) {
	t.Parallel()

	languages := GetSupportedLanguages(nil)

	for _, lang := range languages {
		if lang.Code == "uz-Cyrl" {
			require.Equal(t, "Ўзбекча", lang.VerboseName)
			require.Equal(t, "uz-Cyrl", lang.Tag.String())
			return
		}
	}

	t.Fatalf("expected uz-Cyrl in supported languages, got %+v", languages)
}
