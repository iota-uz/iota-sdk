package controllers

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	"github.com/stretchr/testify/assert"
)

func TestSalaryFieldValue(t *testing.T) {
	t.Parallel()

	zero, amount := 0.0, 1500.5
	assert.Empty(t, salaryFieldValue(&employee.CreateDTO{}))
	assert.Equal(t, "0.00", salaryFieldValue(&employee.CreateDTO{Salary: &zero}))
	assert.Equal(t, "1500.50", salaryFieldValue(&employee.CreateDTO{Salary: &amount}))
}
