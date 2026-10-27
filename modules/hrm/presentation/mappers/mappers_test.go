package mappers_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/modules/hrm/presentation/mappers"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmployeeToViewModel_ResignationDateAndStatus(t *testing.T) {
	t.Parallel()

	email, err := internet.NewEmail("john@example.com")
	require.NoError(t, err)
	build := func(opts ...employee.Option) employee.Employee {
		opts = append(opts, employee.WithBirthDate(time.Date(1990, 5, 20, 0, 0, 0, 0, time.UTC)))
		return employee.New(
			"John", "Doe", "", "+998901234567", email,
			money.NewFromFloat(1000, string(currency.UsdCode)),
			tax.NilTin, tax.NilPin,
			employee.NewLanguage("", ""),
			time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
			opts...,
		)
	}

	active := mappers.EmployeeToViewModel(build())
	assert.Empty(t, active.ResignationDate)
	assert.Equal(t, "active", active.Status)

	resigned := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	former := mappers.EmployeeToViewModel(build(employee.WithResignationDate(&resigned)))
	assert.Equal(t, "2026-09-01", former.ResignationDate)
	assert.Equal(t, "1990-05-20", former.BirthDate)
	assert.Equal(t, "former", former.Status)
}
