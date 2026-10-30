package services_test

import (
	"context"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/hrm"
	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/modules/hrm/domain/entities/position"
	"github.com/iota-uz/iota-sdk/modules/hrm/infrastructure/persistence"
	hrmpermissions "github.com/iota-uz/iota-sdk/modules/hrm/permissions"
	"github.com/iota-uz/iota-sdk/modules/hrm/services"
	"github.com/iota-uz/iota-sdk/modules/logging"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

var allEmployeeFields = []string{
	"birth_date", "email", "first_name", "hire_date", "last_name", "middle_name", "phone", "pin", "resignation_date", "salary", "tin",
}

type hrmHistoryFixture struct {
	env       *itf.TestEnvironment
	employees *services.EmployeeService
	positions *services.PositionService
	events    businessevent.Repository
}

func setupHRMHistory(t *testing.T) hrmHistoryFixture {
	t.Helper()
	env := itf.Setup(t, itf.WithComponents(
		logging.NewComponent(),
		core.NewComponent(&core.ModuleOptions{PermissionSchema: defaults.PermissionSchema()}),
		hrm.NewComponent(),
	), itf.WithUser(itf.User(hrmpermissions.EmployeeRead)))
	require.NoError(t, corepersistence.NewCurrencyRepository().Create(env.Ctx, currency.USD))
	events, err := composition.Resolve[businessevent.Repository](env.Container)
	require.NoError(t, err)
	return hrmHistoryFixture{
		env:       env,
		employees: itf.GetService[services.EmployeeService](env),
		positions: itf.GetService[services.PositionService](env),
		events:    events,
	}
}

func employeeDTO(firstName string) *employee.CreateDTO {
	return &employee.CreateDTO{
		FirstName: firstName,
		LastName:  "Karimov",
		Email:     firstName + "@example.com",
		Phone:     "+99890" + strconv.Itoa(1000000+len(firstName)*7919),
		Salary:    1500.5,
		Pin:       "12345678901234",
		BirthDate: shared.DateOnly(time.Date(1992, 3, 4, 0, 0, 0, 0, time.UTC)),
		HireDate:  shared.DateOnly(time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)),
	}
}

func (f hrmHistoryFixture) newEmployee(t *testing.T, ctx context.Context, firstName string) employee.Employee {
	t.Helper()
	require.NoError(t, f.employees.Create(ctx, employeeDTO(firstName)))
	all, err := f.employees.GetAll(f.env.Ctx)
	require.NoError(t, err)
	for _, e := range all {
		if e.FirstName() == firstName {
			return e
		}
	}
	t.Fatalf("employee %s not created", firstName)
	return nil
}

func (f hrmHistoryFixture) newPosition(t *testing.T, ctx context.Context, name string) *position.Position {
	t.Helper()
	p := &position.Position{TenantID: f.env.Tenant.ID.String(), Name: name, Description: "Keeps the books"}
	require.NoError(t, f.positions.Create(ctx, p))
	return p
}

func (f hrmHistoryFixture) operationEvents(t *testing.T, ctx context.Context) []businessevent.Event {
	t.Helper()
	exec, ok := history.UseExecution(ctx)
	require.True(t, ok)
	events, err := f.events.ListByOperation(f.env.Ctx, exec.OperationID)
	require.NoError(t, err)
	return events
}

func TestHRMHistory_RecordsServiceMutations(t *testing.T) {
	f := setupHRMHistory(t)

	tests := []struct {
		name      string
		act       func(t *testing.T, ctx context.Context) string
		eventType string
		resource  string
		fields    []string
	}{
		{
			name: "employee created",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				return strconv.FormatUint(uint64(f.newEmployee(t, ctx, "Aziz").ID()), 10)
			},
			eventType: services.EmployeeCreated, resource: services.EmployeeResource,
			fields: allEmployeeFields,
		},
		{
			name: "employee salary raised and resigned",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				e := f.newEmployee(t, f.env.Ctx, "Bobur")
				dto := employeeDTO("Bobur")
				require.NoError(t, f.employees.Update(ctx, e.ID(), &employee.UpdateDTO{
					FirstName: dto.FirstName, LastName: dto.LastName, Email: dto.Email, Phone: dto.Phone,
					Salary: 2000, Pin: dto.Pin, BirthDate: dto.BirthDate, HireDate: dto.HireDate,
					ResignationDate: shared.DateOnly(time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)),
				}))
				return strconv.FormatUint(uint64(e.ID()), 10)
			},
			eventType: services.EmployeeUpdated, resource: services.EmployeeResource,
			fields: []string{"resignation_date", "salary"},
		},
		{
			name: "employee deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				e := f.newEmployee(t, f.env.Ctx, "Dilnoza")
				_, err := f.employees.Delete(ctx, e.ID())
				require.NoError(t, err)
				return strconv.FormatUint(uint64(e.ID()), 10)
			},
			eventType: services.EmployeeDeleted, resource: services.EmployeeResource,
			fields: allEmployeeFields,
		},
		{
			name: "position created",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				return strconv.FormatUint(uint64(f.newPosition(t, ctx, "Accountant").ID), 10)
			},
			eventType: services.PositionCreated, resource: services.PositionResource,
			fields: []string{"description", "name"},
		},
		{
			name: "position renamed",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				p := f.newPosition(t, f.env.Ctx, "Cashier")
				p.Name = "Senior cashier"
				require.NoError(t, f.positions.Update(ctx, p))
				return strconv.FormatUint(uint64(p.ID), 10)
			},
			eventType: services.PositionUpdated, resource: services.PositionResource,
			fields: []string{"name"},
		},
		{
			name: "position deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				p := f.newPosition(t, f.env.Ctx, "Courier")
				require.NoError(t, f.positions.Delete(ctx, int64(p.ID)))
				return strconv.FormatUint(uint64(p.ID), 10)
			},
			eventType: services.PositionDeleted, resource: services.PositionResource,
			fields: []string{"description", "name"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := itf.TestExecution(f.env.Ctx)
			id := tt.act(t, ctx)

			events := f.operationEvents(t, ctx)
			require.Len(t, events, 1)
			e := events[0]
			require.Equal(t, tt.eventType, e.Type)
			require.Equal(t, []businessevent.Subject{{Resource: tt.resource, ID: id}}, e.Subjects)
			require.Len(t, e.Changes, 1)
			fields := make([]string, 0, len(e.Changes[0].Fields))
			for _, d := range e.Changes[0].Fields {
				fields = append(fields, d.Field)
			}
			sort.Strings(fields)
			require.Equal(t, tt.fields, fields)
		})
	}
}

func TestHRMHistory_SalaryChangeKeepsValues(t *testing.T) {
	f := setupHRMHistory(t)
	e := f.newEmployee(t, f.env.Ctx, "Farrukh")
	dto := employeeDTO("Farrukh")
	ctx := itf.TestExecution(f.env.Ctx)

	require.NoError(t, f.employees.Update(ctx, e.ID(), &employee.UpdateDTO{
		FirstName: dto.FirstName, LastName: dto.LastName, Email: dto.Email, Phone: dto.Phone,
		Salary: 2000, Pin: dto.Pin, BirthDate: dto.BirthDate, HireDate: dto.HireDate,
	}))

	events := f.operationEvents(t, ctx)
	require.Len(t, events, 1)
	require.Len(t, events[0].Changes[0].Fields, 1)
	d := events[0].Changes[0].Fields[0]
	require.Equal(t, "salary", d.Field)
	require.JSONEq(t, `{"amount":"1500.50","currency":"USD"}`, string(d.Before.Value))
	require.JSONEq(t, `{"amount":"2000.00","currency":"USD"}`, string(d.After.Value))
}

func TestHRMHistory_UnchangedUpdateRecordsNothing(t *testing.T) {
	f := setupHRMHistory(t)
	p := f.newPosition(t, f.env.Ctx, "Driver")
	ctx := itf.TestExecution(f.env.Ctx)

	require.NoError(t, f.positions.Update(ctx, p))

	require.Empty(t, f.operationEvents(t, ctx))
}

func TestHRMHistory_RollbackLeavesNoEvents(t *testing.T) {
	f := setupHRMHistory(t)

	tests := []struct {
		name    string
		ctx     func(operation context.Context, tx pgx.Tx) context.Context
		wantErr error
	}{
		{
			name: "rolled back operation",
			ctx:  func(operation context.Context, _ pgx.Tx) context.Context { return operation },
		},
		{
			name: "no execution context",
			ctx: func(_ context.Context, tx pgx.Tx) context.Context {
				ctx := composables.WithPool(context.Background(), f.env.Pool)
				ctx = composables.WithTenantID(ctx, f.env.Tenant.ID)
				ctx = composables.WithParams(ctx, itf.DefaultParams())
				ctx = composables.WithSession(ctx, itf.MockSession())
				ctx = composables.WithUser(ctx, f.env.User)
				return composables.WithTx(ctx, tx)
			},
			wantErr: history.ErrExecutionMissing,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			savepoint, err := f.env.Tx.Begin(f.env.Ctx)
			require.NoError(t, err)
			operation := itf.TestExecution(composables.WithTx(f.env.Ctx, savepoint))
			p := &position.Position{TenantID: f.env.Tenant.ID.String(), Name: "Rolled back"}

			err = f.positions.Create(tt.ctx(operation, savepoint), p)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				require.Len(t, f.operationEvents(t, operation), 1)
			}
			require.NoError(t, savepoint.Rollback(f.env.Ctx))

			require.Empty(t, f.operationEvents(t, operation))
			_, err = f.positions.GetByID(f.env.Ctx, int64(p.ID))
			require.ErrorIs(t, err, persistence.ErrPositionNotFound)
		})
	}
}
