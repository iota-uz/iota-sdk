package dtos

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/pkg/crud/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requiredLocales mirrors services.orgRequiredLocales: the seeder/service
// validation rejects a name missing any of these, so the form→entity path must
// preserve every locale the form submits (notably the cased "uz-Cyrl").
var requiredLocales = []string{"en", "ru", "uz", "uz-Cyrl"}

func mustML(t *testing.T) models.MultiLang {
	t.Helper()
	ml, err := models.NewMultiLangFromMap(map[string]string{"en": "x", "ru": "x", "uz": "x", "uz-Cyrl": "x"})
	require.NoError(t, err)
	return ml
}

func TestCreateDepartmentDTO_ToEntity_RoundTripsAllLocales(t *testing.T) {
	t.Parallel()
	dto := &CreateDepartmentDTO{
		Name:   map[string]string{"en": "Legal", "ru": "Юридический", "uz": "Yuridik", "uz-Cyrl": "Юридик"},
		Code:   "legal",
		Order:  5,
		Status: "active",
	}

	ent, err := dto.ToEntity()
	require.NoError(t, err)

	assert.Equal(t, "legal", ent.Code())
	assert.Equal(t, 5, ent.Order())
	assert.Equal(t, department.StatusActive, ent.Status())
	assert.Nil(t, ent.ParentID())
	for _, loc := range requiredLocales {
		assert.Truef(t, ent.NameI18n().HasLocale(loc), "entity name missing locale %s", loc)
	}
}

func TestCreateDepartmentDTO_ToEntity_WithParent(t *testing.T) {
	t.Parallel()
	pid := uuid.New()
	dto := &CreateDepartmentDTO{
		Name:     map[string]string{"en": "X", "ru": "X", "uz": "X", "uz-Cyrl": "X"},
		Code:     "x",
		ParentID: pid.String(),
		Status:   "active",
	}

	ent, err := dto.ToEntity()
	require.NoError(t, err)
	require.NotNil(t, ent.ParentID())
	assert.Equal(t, pid, *ent.ParentID())
}

func TestUpdateDepartmentDTO_Apply_RoundTripsAllLocales(t *testing.T) {
	t.Parallel()
	base := department.New("old", mustML(t))
	dto := &UpdateDepartmentDTO{
		Name:   map[string]string{"en": "New", "ru": "Новый", "uz": "Yangi", "uz-Cyrl": "Янги"},
		Code:   "new",
		Order:  3,
		Status: "inactive",
	}

	ent, err := dto.Apply(base)
	require.NoError(t, err)

	assert.Equal(t, "new", ent.Code())
	assert.Equal(t, 3, ent.Order())
	assert.Equal(t, department.StatusInactive, ent.Status())
	for _, loc := range requiredLocales {
		assert.Truef(t, ent.NameI18n().HasLocale(loc), "entity name missing locale %s", loc)
	}
}
