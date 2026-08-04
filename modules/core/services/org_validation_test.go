package services

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/crud/models"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stretchr/testify/require"
)

func TestValidateOrgMultiLang(t *testing.T) {
	t.Parallel()

	const op serrors.Op = "test"

	t.Run("all required locales present", func(t *testing.T) {
		t.Parallel()
		ml, err := models.NewMultiLangFromMap(map[string]string{
			"en":      "Engineering",
			"ru":      "Инженерия",
			"uz":      "Muhandislik",
			"uz-Cyrl": "Муҳандислик",
		})
		require.NoError(t, err)
		require.NoError(t, validateOrgMultiLang(op, "name", ml))
	})

	t.Run("nil value rejected", func(t *testing.T) {
		t.Parallel()
		require.Error(t, validateOrgMultiLang(op, "name", nil))
	})

	t.Run("missing locale rejected", func(t *testing.T) {
		t.Parallel()
		ml, err := models.NewMultiLangFromMap(map[string]string{
			"en": "Engineering",
			"ru": "Инженерия",
			"uz": "Muhandislik",
			// uz-Cyrl intentionally omitted
		})
		require.NoError(t, err)
		err = validateOrgMultiLang(op, "name", ml)
		require.Error(t, err)
	})

	t.Run("empty value rejected", func(t *testing.T) {
		t.Parallel()
		ml, err := models.NewMultiLangFromMap(map[string]string{})
		require.NoError(t, err)
		require.Error(t, validateOrgMultiLang(op, "title", ml))
	})
}
