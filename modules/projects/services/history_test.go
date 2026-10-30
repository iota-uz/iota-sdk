package services_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	financepermissions "github.com/iota-uz/iota-sdk/modules/finance/permissions"
	financeservices "github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/modules/logging"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/projects"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	projectstage "github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project_stage"
	"github.com/iota-uz/iota-sdk/modules/projects/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type projectsHistoryFixture struct {
	env         *itf.TestEnvironment
	client      uuid.UUID
	projects    *services.ProjectService
	stages      *services.ProjectStageService
	acceptances *services.AcceptanceService
	events      businessevent.Repository
}

func setupProjectsHistory(t *testing.T) projectsHistoryFixture {
	t.Helper()
	env := itf.Setup(t, itf.WithComponents(
		logging.NewComponent(),
		core.NewComponent(&core.ModuleOptions{PermissionSchema: defaults.PermissionSchema()}),
		finance.NewComponent(),
		projects.NewComponent(),
	), itf.WithUser(itf.User(financepermissions.PaymentCreate)))
	require.NoError(t, itf.GetService[coreservices.CurrencyService](env).Create(env.Ctx, &currency.CreateDTO{
		Code: string(currency.USD.Code()), Name: currency.USD.Name(), Symbol: string(currency.USD.Symbol()),
	}))
	client, err := itf.GetService[financeservices.CounterpartyService](env).Create(env.Ctx, counterparty.New(
		"History Client", counterparty.Customer, counterparty.LLC, counterparty.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)
	events, err := composition.Resolve[businessevent.Repository](env.Container)
	require.NoError(t, err)
	return projectsHistoryFixture{
		env:         env,
		client:      client.ID(),
		projects:    itf.GetService[services.ProjectService](env),
		stages:      itf.GetService[services.ProjectStageService](env),
		acceptances: itf.GetService[services.AcceptanceService](env),
		events:      events,
	}
}

func (f projectsHistoryFixture) newProject(t *testing.T) project.Project {
	t.Helper()
	p := project.New("Office "+uuid.NewString(), f.client, project.WithTenantID(f.env.Tenant.ID), project.WithContract(money.New(100000, "USD")))
	require.NoError(t, f.projects.Create(f.env.Ctx, p))
	return p
}

func (f projectsHistoryFixture) newStage(t *testing.T) projectstage.ProjectStage {
	t.Helper()
	stage := projectstage.New(f.newProject(t).ID(), 1, 5000)
	require.NoError(t, f.stages.Create(f.env.Ctx, stage))
	return stage
}

func (f projectsHistoryFixture) newDocument(t *testing.T) acceptance.Document {
	t.Helper()
	document, err := f.acceptances.Create(f.env.Ctx, acceptance.New(
		f.newProject(t).ID(), acceptance.KindAct, "A-1", time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), money.New(40000, "USD"),
		acceptance.WithTenantID(f.env.Tenant.ID),
	))
	require.NoError(t, err)
	return document
}

func (f projectsHistoryFixture) operationEvents(t *testing.T, ctx context.Context) []businessevent.Event {
	t.Helper()
	exec, ok := history.UseExecution(ctx)
	require.True(t, ok)
	events, err := f.events.ListByOperation(f.env.Ctx, exec.OperationID)
	require.NoError(t, err)
	return events
}

func TestProjectsHistory_RecordsServiceMutations(t *testing.T) {
	f := setupProjectsHistory(t)
	date := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		act       func(t *testing.T, ctx context.Context) string
		eventType string
		resource  string
		fields    []string
	}{
		{
			name: "project created by upsert of a new ID",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				p := project.New("Warehouse", f.client, project.WithTenantID(f.env.Tenant.ID), project.WithContract(money.New(250000, "USD")))
				require.NotEqual(t, uuid.Nil, p.ID())
				require.NoError(t, f.projects.Create(ctx, p))
				return p.ID().String()
			},
			eventType: services.ProjectCreated, resource: services.ProjectResource,
			fields: []string{"contract", "counterparty_id", "description", "name"},
		},
		{
			name: "project updated",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				p := f.newProject(t)
				require.NoError(t, f.projects.Update(ctx, p.UpdateName("Office 2").UpdateDescription("Second floor")))
				return p.ID().String()
			},
			eventType: services.ProjectUpdated, resource: services.ProjectResource,
			fields: []string{"description", "name"},
		},
		{
			name: "project deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				p := f.newProject(t)
				_, err := f.projects.Delete(ctx, p.ID())
				require.NoError(t, err)
				return p.ID().String()
			},
			eventType: services.ProjectDeleted, resource: services.ProjectResource,
			fields: []string{"contract", "counterparty_id", "description", "name"},
		},
		{
			name: "project stage created",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				stage := projectstage.New(f.newProject(t).ID(), 0, 7000, projectstage.WithStartDate(&date))
				require.NoError(t, f.stages.Create(ctx, stage))
				return stage.ID().String()
			},
			eventType: services.ProjectStageCreated, resource: services.ProjectStageResource,
			fields: []string{"description", "factual_end_date", "planned_end_date", "project_id", "stage_number", "start_date", "total_amount"},
		},
		{
			name: "project stage updated",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				stage := f.newStage(t)
				require.NoError(t, f.stages.Update(ctx, stage.UpdateTotalAmount(9000).UpdateFactualEndDate(&date)))
				return stage.ID().String()
			},
			eventType: services.ProjectStageUpdated, resource: services.ProjectStageResource,
			fields: []string{"factual_end_date", "total_amount"},
		},
		{
			name: "project stage deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				stage := f.newStage(t)
				_, err := f.stages.Delete(ctx, stage.ID())
				require.NoError(t, err)
				return stage.ID().String()
			},
			eventType: services.ProjectStageDeleted, resource: services.ProjectStageResource,
			fields: []string{"description", "factual_end_date", "planned_end_date", "project_id", "stage_number", "start_date", "total_amount"},
		},
		{
			name: "acceptance document created",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				document, err := f.acceptances.Create(ctx, acceptance.New(
					f.newProject(t).ID(), acceptance.KindDeliveryNote, "N-7", date, money.New(12000, "USD"),
					acceptance.WithTenantID(f.env.Tenant.ID),
				))
				require.NoError(t, err)
				return document.ID().String()
			},
			eventType: services.AcceptanceCreated, resource: services.AcceptanceResource,
			fields: []string{"amount", "date", "description", "kind", "number", "project_id", "status"},
		},
		{
			name: "acceptance document signed",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				document := f.newDocument(t)
				_, err := f.acceptances.Sign(ctx, document.ProjectID(), document.ID())
				require.NoError(t, err)
				return document.ID().String()
			},
			eventType: services.AcceptanceSigned, resource: services.AcceptanceResource,
			fields: []string{"status"},
		},
		{
			name: "acceptance document cancelled",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				document := f.newDocument(t)
				_, err := f.acceptances.Cancel(ctx, document.ProjectID(), document.ID())
				require.NoError(t, err)
				return document.ID().String()
			},
			eventType: services.AcceptanceCancelled, resource: services.AcceptanceResource,
			fields: []string{"status"},
		},
		{
			name: "acceptance document deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				document := f.newDocument(t)
				require.NoError(t, f.acceptances.Delete(ctx, document.ProjectID(), document.ID()))
				return document.ID().String()
			},
			eventType: services.AcceptanceDeleted, resource: services.AcceptanceResource,
			fields: []string{"amount", "date", "description", "kind", "number", "project_id", "status"},
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

func TestProjectsHistory_UnchangedUpdateRecordsNothing(t *testing.T) {
	f := setupProjectsHistory(t)
	p := f.newProject(t)
	ctx := itf.TestExecution(f.env.Ctx)

	require.NoError(t, f.projects.Update(ctx, p))

	require.Empty(t, f.operationEvents(t, ctx))
}

func TestProjectsHistory_RollbackLeavesNoEvents(t *testing.T) {
	f := setupProjectsHistory(t)

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
			p := project.New("Rolled back", f.client, project.WithTenantID(f.env.Tenant.ID))

			err = f.projects.Create(tt.ctx(operation, savepoint), p)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				require.Len(t, f.operationEvents(t, operation), 1)
			}
			require.NoError(t, savepoint.Rollback(f.env.Ctx))

			require.Empty(t, f.operationEvents(t, operation))
			_, err = f.projects.GetByID(f.env.Ctx, p.ID())
			require.ErrorIs(t, err, persistence.ErrProjectNotFound)
		})
	}
}
