package employee_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/assert"
)

func newEmployee(opts ...employee.Option) employee.Employee {
	return employee.New(
		"John", "Doe", "", "+998901234567", nil,
		money.NewFromFloat(1000, string(currency.UsdCode)),
		tax.NilTin, tax.NilPin,
		employee.NewLanguage("", ""),
		time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
		opts...,
	)
}

func TestEmployee_Status(t *testing.T) {
	t.Parallel()

	resigned := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	assert.Equal(t, employee.StatusActive, newEmployee().Status())
	assert.Equal(t, employee.StatusFormer, newEmployee(employee.WithResignationDate(&resigned)).Status())
	assert.Equal(t, employee.StatusFormer, newEmployee().MarkAsResigned(resigned).Status())
}

func TestParseStatus(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in   string
		want employee.Status
		ok   bool
	}{
		{"active", employee.StatusActive, true},
		{"former", employee.StatusFormer, true},
		{"", "", false},
		{"fired", "", false},
	} {
		got, ok := employee.ParseStatus(tc.in)
		assert.Equal(t, tc.want, got, tc.in)
		assert.Equal(t, tc.ok, ok, tc.in)
	}
}
