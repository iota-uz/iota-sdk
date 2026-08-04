package mappers

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/userposition"
	"github.com/iota-uz/iota-sdk/pkg/crud/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requiredLocales mirrors services.orgRequiredLocales: the edit form renders one
// input per locale via Department.NameForLocale / UserPosition.TitleForLocale, so
// every required locale (notably the cased "uz-Cyrl") must survive the
// entity -> viewmodel round-trip or the input renders empty and the save drops it.
var requiredLocales = []string{"en", "ru", "uz", "uz-Cyrl"}

func TestDepartmentToViewModel_NameForLocaleRoundTripsAllLocales(t *testing.T) {
	t.Parallel()
	name, err := models.NewMultiLangFromMap(map[string]string{
		"en":      "Legal",
		"ru":      "Юридический",
		"uz":      "Yuridik",
		"uz-Cyrl": "Юридик",
	})
	require.NoError(t, err)

	entity := department.New("legal", name)
	vm := DepartmentToViewModel(entity, "en", nil)

	// Every UI locale code must resolve, including the cased "uz-Cyrl" that
	// MultiLang.GetAll normalizes to "uz-cyrl".
	for _, loc := range requiredLocales {
		assert.NotEmptyf(t, vm.NameForLocale(loc), "NameForLocale(%q) should round-trip", loc)
	}
	assert.Equal(t, "Юридик", vm.NameForLocale("uz-Cyrl"))
}

func TestUserPositionToViewModel_TitleForLocaleRoundTripsAllLocales(t *testing.T) {
	t.Parallel()
	title, err := models.NewMultiLangFromMap(map[string]string{
		"en":      "Lawyer",
		"ru":      "Юрист",
		"uz":      "Yurist",
		"uz-Cyrl": "Юрист",
	})
	require.NoError(t, err)

	entity := userposition.New(1, uuid.New(), title)
	vm := UserPositionToViewModel(entity, "en", nil, nil)

	for _, loc := range requiredLocales {
		assert.NotEmptyf(t, vm.TitleForLocale(loc), "TitleForLocale(%q) should round-trip", loc)
	}
	assert.Equal(t, "Юрист", vm.TitleForLocale("uz-Cyrl"))
}
