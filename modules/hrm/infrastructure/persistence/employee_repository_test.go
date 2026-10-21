package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/modules/hrm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createEmployee(t *testing.T, ctx context.Context, repo employee.Repository, firstName, mail string, resignation *time.Time) employee.Employee {
	t.Helper()

	email, err := internet.NewEmail(mail)
	require.NoError(t, err)
	created, err := repo.Create(ctx, employee.New(
		firstName, "Doe", "", "", email,
		money.NewFromFloat(1000, string(currency.UsdCode)),
		tax.NilTin, tax.NilPin,
		employee.NewLanguage("", ""),
		time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
		employee.WithResignationDate(resignation),
	))
	require.NoError(t, err)
	return created
}

func firstNames(employees []employee.Employee) []string {
	names := make([]string, 0, len(employees))
	for _, e := range employees {
		names = append(names, e.FirstName())
	}
	return names
}

func TestEmployeeRepository_GetPaginated_FiltersByStatusWithinTenant(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	require.NoError(t, corepersistence.NewCurrencyRepository().Create(f.Ctx, currency.USD))
	repo := persistence.NewEmployeeRepository()

	resigned := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	createEmployee(t, f.Ctx, repo, "Alice", "alice@example.com", nil)
	createEmployee(t, f.Ctx, repo, "Bob", "bob@example.com", &resigned)

	otherTenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
	require.NoError(t, err)
	otherCtx := composables.WithTenantID(f.Ctx, otherTenant.ID)
	createEmployee(t, otherCtx, repo, "Carol", "carol@example.com", nil)
	createEmployee(t, otherCtx, repo, "Dave", "dave@example.com", &resigned)

	for _, tc := range []struct {
		name   string
		status employee.Status
		want   []string
	}{
		{"all", "", []string{"Alice", "Bob"}},
		{"active", employee.StatusActive, []string{"Alice"}},
		{"former", employee.StatusFormer, []string{"Bob"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.GetPaginated(f.Ctx, &employee.FindParams{Status: tc.status})
			require.NoError(t, err)
			assert.ElementsMatch(t, tc.want, firstNames(got))
		})
	}

	got, err := repo.GetPaginated(otherCtx, &employee.FindParams{Status: employee.StatusFormer})
	require.NoError(t, err)
	assert.Equal(t, []string{"Dave"}, firstNames(got))
}

func TestEmployeeRepository_Update_ResignationDateChangesStatus(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	require.NoError(t, corepersistence.NewCurrencyRepository().Create(f.Ctx, currency.USD))
	repo := persistence.NewEmployeeRepository()

	created := createEmployee(t, f.Ctx, repo, "Alice", "alice@example.com", nil)
	require.Equal(t, employee.StatusActive, created.Status())

	require.NoError(t, repo.Update(f.Ctx, created.MarkAsResigned(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))))
	resigned, err := repo.GetByID(f.Ctx, created.ID())
	require.NoError(t, err)
	assert.Equal(t, employee.StatusFormer, resigned.Status())

	rehired := employee.NewWithID(
		resigned.ID(), resigned.TenantID(),
		resigned.FirstName(), resigned.LastName(), resigned.MiddleName(), resigned.Phone(),
		resigned.Email(), resigned.Salary(), resigned.Tin(), resigned.Pin(),
		resigned.Language(), resigned.HireDate(),
	)
	require.NoError(t, repo.Update(f.Ctx, rehired))
	got, err := repo.GetByID(f.Ctx, created.ID())
	require.NoError(t, err)
	assert.Equal(t, employee.StatusActive, got.Status())
}
