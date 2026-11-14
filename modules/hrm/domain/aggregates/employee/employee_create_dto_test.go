package employee_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		salary  []string
		wantErr bool
		want    float64
	}{
		{name: "omitted is rejected", salary: nil, wantErr: true},
		{name: "blank is rejected", salary: []string{""}, wantErr: true},
		{name: "zero is allowed", salary: []string{"0"}, want: 0},
		{name: "zero with fraction is allowed", salary: []string{"0.00"}, want: 0},
		{name: "positive is allowed", salary: []string{"1500000.5"}, want: 1500000.5},
		{name: "negative is rejected", salary: []string{"-1"}, wantErr: true, want: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			values := url.Values{
				"FirstName": {"John"},
				"LastName":  {"Doe"},
				"Email":     {"john@example.com"},
				"Phone":     {"+998901234567"},
			}
			if tc.salary != nil {
				values["Salary"] = tc.salary
			}
			dto := &employee.CreateDTO{}
			require.NoError(t, shared.Decoder.Decode(dto, values))

			errs, ok := dto.Ok(ctx)
			assert.Equal(t, !tc.wantErr, ok, "errors: %v", errs)
			if tc.wantErr {
				assert.Contains(t, errs, "Salary")
				return
			}
			assert.NotContains(t, errs, "Salary")
			assert.InDelta(t, tc.want, dto.SalaryAmount(), 0)
		})
	}
}
