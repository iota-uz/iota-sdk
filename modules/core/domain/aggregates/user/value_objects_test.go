package user

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewUILanguageAcceptsUzbekCyrillic(t *testing.T) {
	language, err := NewUILanguage("uz-Cyrl")

	require.NoError(t, err)
	require.Equal(t, UILanguageUZCyrl, language)
}
