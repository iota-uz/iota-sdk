package employee_test

import (
	"context"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

func TestCreateDTO_Salary(t *testing.T) {
	t.Parallel()

	bundle := i18n.NewBundle(language.English)
	bundle.MustAddMessages(language.English,
		&i18n.Message{ID: "ValidationErrors.required", Other: "required"},
		&i18n.Message{ID: "ValidationErrors.gte", Other: "gte"},
	)
	ctx := intl.WithLocalizer(context.Background(), i18n.NewLocalizer(bundle, "en"))

	for _, tc := range []struct {
		name    string
		salary  float64
		wantErr bool
	}{
		{name: "zero is allowed", salary: 0},
		{name: "positive is allowed", salary: 1500000.5},
		{name: "negative is rejected", salary: -1, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dto := &employee.CreateDTO{
				FirstName: "John",
				LastName:  "Doe",
				Email:     "john@example.com",
				Phone:     "+998901234567",
				Salary:    tc.salary,
			}
			errs, ok := dto.Ok(ctx)
			assert.Equal(t, !tc.wantErr, ok, "errors: %v", errs)
			if tc.wantErr {
				assert.Contains(t, errs, "Salary")
			} else {
				assert.NotContains(t, errs, "Salary")
			}
		})
	}
}
