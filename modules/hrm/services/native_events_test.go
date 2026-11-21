package services_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/hrm"
	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	hrmpermissions "github.com/iota-uz/iota-sdk/modules/hrm/permissions"
	"github.com/iota-uz/iota-sdk/modules/hrm/services"
	"github.com/iota-uz/iota-sdk/modules/logging"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

func TestEmployeeEvents_CommitAndRollBackWithTheEmployee(t *testing.T) {
	env := itf.Setup(t, itf.WithComponents(
		logging.NewComponent(),
		core.NewComponent(&core.ModuleOptions{PermissionSchema: defaults.PermissionSchema()}),
		hrm.NewComponent(),
	), itf.WithUser(itf.User(hrmpermissions.EmployeeRead)))
	require.NoError(t, corepersistence.NewCurrencyRepository().Create(env.Ctx, currency.USD))
	actor, err := corepersistence.NewUserRepository(corepersistence.NewUploadRepository()).Create(env.Ctx,
		user.New("HR", "Manager", internet.MustParseEmail("hr-"+uuid.NewString()+"@example.com"), user.UILanguageEN, user.WithTenantID(env.TenantID())))
	require.NoError(t, err)
	ctx := composables.WithUser(env.Ctx, actor)
	employees := itf.GetService[services.EmployeeService](env)
	ids := []string{services.EmployeeCreatedEventID, services.EmployeeUpdatedEventID}
	phones := map[string]string{"rolled-back": "+998901110001", "committed": "+998901110002"}
	hired := shared.DateOnly(time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC))
	salary := 1500.5

	commits := func(want map[string]int, change func(ctx context.Context, suffix string) error) map[string][]itf.OutboxRow {
		got := itf.OutboxCommitsAndRollsBack(t, ctx, env.TenantID(), ids, want, change)
		for id, rows := range got {
			for _, r := range rows {
				require.NotNil(t, r.Initiator, "%s: the acting user is the event initiator", id)
				require.EqualValues(t, actor.ID(), *r.Initiator)
				require.Equal(t, "hrm.employee", r.SubjectEntity)
			}
		}
		return got
	}

	var created itf.OutboxRow
	t.Run("created", func(t *testing.T) {
		got := commits(map[string]int{services.EmployeeCreatedEventID: 1}, func(ctx context.Context, suffix string) error {
			return employees.Create(ctx, &employee.CreateDTO{
				FirstName: "Aziz", LastName: "Karimov", Email: suffix + "@example.com", Phone: phones[suffix], Salary: &salary,
				Pin: "12345678901234", BirthDate: shared.DateOnly(time.Date(1992, 3, 4, 0, 0, 0, 0, time.UTC)), HireDate: hired,
			})
		})
		created = got[services.EmployeeCreatedEventID][0]
		var p services.EmployeeCreatedPayload
		require.NoError(t, json.Unmarshal(created.Payload, &p))
		require.Equal(t, fmt.Sprint(p.EmployeeID), created.SubjectKey)
		require.Equal(t, "Aziz", p.FirstName)
		require.Equal(t, "Karimov", p.LastName)
		require.Equal(t, "committed@example.com", p.Email)
		require.Equal(t, "+998901110002", *p.Phone)
		require.Equal(t, "2025-01-10", p.HireDate)
		for _, private := range []string{"salary", "tin", "pin", "birthDate", "1500", "12345678901234", "1992"} {
			require.NotContains(t, strings.ToLower(string(created.Payload)), strings.ToLower(private), "%s never travels", private)
		}
	})

	var employeeID uint
	all, err := employees.GetAll(env.Ctx)
	require.NoError(t, err)
	for _, e := range all {
		if e.Email().Value() == "committed@example.com" {
			employeeID = e.ID()
		}
	}
	require.NotZero(t, employeeID)

	update := func(salary float64, resigned shared.DateOnly) *employee.UpdateDTO {
		return &employee.UpdateDTO{
			FirstName: "Aziz", LastName: "Karimov", Email: "committed@example.com", Phone: "+998901110002",
			Salary: salary, Pin: "12345678901234", BirthDate: shared.DateOnly(time.Date(1992, 3, 4, 0, 0, 0, 0, time.UTC)),
			HireDate: hired, ResignationDate: resigned,
		}
	}

	t.Run("updated", func(t *testing.T) {
		resigned := shared.DateOnly(time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
		got := commits(map[string]int{services.EmployeeUpdatedEventID: 1}, func(ctx context.Context, _ string) error {
			return employees.Update(ctx, employeeID, update(2000, resigned))
		})
		row := got[services.EmployeeUpdatedEventID][0]
		require.Equal(t, fmt.Sprint(employeeID), row.SubjectKey)
		require.JSONEq(t, fmt.Sprintf(`{"employeeId":"%d","changed":["resignationDate","salary"],"resignationDate":"2026-09-30"}`, employeeID), string(row.Payload),
			"the changed fields are named, their values are not sent")

		// An update that changes nothing publishes nothing.
		before := itf.OutboxCounts(t, ctx, env.TenantID(), ids...)
		require.NoError(t, employees.Update(ctx, employeeID, update(2000, resigned)))
		require.Equal(t, before, itf.OutboxCounts(t, ctx, env.TenantID(), ids...))
	})
}
