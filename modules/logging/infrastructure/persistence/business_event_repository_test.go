package persistence_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/logging/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
	"github.com/stretchr/testify/require"
)

var salaryRead = permission.MustCreate(uuid.MustParse("4d0f7f8e-46a5-4a5e-9a26-0d6f1f8b2d11"), "Employees.Salary.Read", "employees", permission.ActionRead, permission.ModifierAll)

func employeeDefinition(version int, fields ...history.Field) history.Definition {
	return history.Definition{
		Type:           "hrm.employee.updated",
		Version:        version,
		Category:       businessevent.CategoryBusiness,
		TitleKey:       "Employees.History.Updated.Title",
		DescriptionKey: "Employees.History.Updated.Description",
		Resources:      []history.Resource{{Name: "employee", Fields: fields}},
	}
}

func employeeV1() history.Definition {
	return employeeDefinition(1,
		history.Field{Name: "position", LabelKey: "Employees.Fields.Position", Type: history.ValueString},
		history.Field{Name: "salary", LabelKey: "Employees.Fields.Salary", Type: history.ValueMoney, ReadPermission: &salaryRead},
		history.Field{Name: "pin", LabelKey: "Employees.Fields.PIN", Type: history.ValueString, Redact: true},
	)
}

func newWriter(t *testing.T, defs ...history.Definition) history.Writer {
	t.Helper()
	registry := history.NewRegistry()
	require.NoError(t, registry.Register(defs...))
	return history.NewWriter(registry, persistence.NewBusinessEventRepository())
}

func createUser(t *testing.T, ctx context.Context, first, last, middle, email string) user.User {
	t.Helper()
	tenantID, err := composables.UseTenantID(ctx)
	require.NoError(t, err)
	addr, err := internet.NewEmail(email)
	require.NoError(t, err)
	u, err := corepersistence.NewUserRepository(corepersistence.NewUploadRepository()).Create(ctx,
		user.New(first, last, addr, user.UILanguageRU, user.WithTenantID(tenantID), user.WithMiddleName(middle)))
	require.NoError(t, err)
	return u
}

func asUser(ctx context.Context, u user.User, operationID string) context.Context {
	return history.WithExecution(composables.WithUser(ctx, u), history.Execution{Channel: businessevent.ChannelUI, OperationID: operationID})
}

func value(raw string) fielddiff.FieldValue {
	return fielddiff.Present(json.RawMessage(raw))
}

func employeeUpdate(id string, fields ...fielddiff.Diff) history.Input {
	return history.Input{
		Type:     "hrm.employee.updated",
		Version:  1,
		Outcome:  businessevent.OutcomeSucceeded,
		Subjects: []businessevent.Subject{{Resource: "employee", ID: id}},
		Changes:  []businessevent.Change{{Resource: "employee", ID: id, Fields: fields}},
	}
}

func positionChange(before, after string) fielddiff.Diff {
	return fielddiff.Diff{Field: "position", Before: value(strconv.Quote(before)), After: value(strconv.Quote(after))}
}

func TestBusinessEventRepository_ActorSnapshotOutlivesUser(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	users := corepersistence.NewUserRepository(corepersistence.NewUploadRepository())
	actor := createUser(t, f.Ctx, "Ali", "Valiyev", "Karimovich", "history-actor@example.com")
	repo := persistence.NewBusinessEventRepository()

	id, err := newWriter(t, employeeV1()).Record(asUser(f.Ctx, actor, "op-rename"), employeeUpdate("10", positionChange("Accountant", "Chief accountant")))
	require.NoError(t, err)

	require.NoError(t, users.Update(f.Ctx, actor.SetName("Vali", "Aliyev", "")))
	renamed, err := repo.Get(f.Ctx, id)
	require.NoError(t, err)
	require.Equal(t, "Ali", renamed.Actor.FirstName)
	require.Equal(t, "Valiyev", renamed.Actor.LastName)
	require.Equal(t, "Karimovich", renamed.Actor.MiddleName)

	require.NoError(t, users.Delete(f.Ctx, actor.ID()))
	_, err = users.GetByID(f.Ctx, actor.ID())
	require.Error(t, err)
	deleted, err := repo.Get(f.Ctx, id)
	require.NoError(t, err)
	require.Equal(t, &businessevent.ActorSnapshot{
		SchemaVersion: 1, Kind: businessevent.ActorUser, UserID: strconv.FormatUint(uint64(actor.ID()), 10),
		FirstName: "Ali", LastName: "Valiyev", MiddleName: "Karimovich",
	}, deleted.Actor)

	var byActor int
	require.NoError(t, f.Tx.QueryRow(f.Ctx, `SELECT count(*) FROM action_logs WHERE tenant_id = $1 AND actor_kind = 'user' AND actor_id = $2`,
		f.TenantID(), strconv.FormatUint(uint64(actor.ID()), 10)).Scan(&byActor))
	require.Equal(t, 1, byActor)
}

func TestBusinessEventRepository_LegacyActionLogsReadAsIncomplete(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	known := createUser(t, f.Ctx, "Old", "Operator", "", "legacy-operator@example.com")
	var withUser, anonymous int
	var withUserID, anonymousID uuid.UUID
	require.NoError(t, f.Tx.QueryRow(f.Ctx, `INSERT INTO action_logs (tenant_id, method, path, user_id, before, after, user_agent, ip, created_at)
		VALUES ($1, 'POST', '/finance/payments/5', $2, '{"amount": 10}', '{"amount": 12}', 'Mozilla', '10.0.0.1', '2025-01-02T03:04:05Z') RETURNING id, event_id`,
		f.TenantID(), known.ID()).Scan(&withUser, &withUserID))
	require.NoError(t, f.Tx.QueryRow(f.Ctx, `INSERT INTO action_logs (tenant_id, method, path, user_agent, ip, created_at)
		VALUES ($1, 'DELETE', '/finance/expenses/7', 'curl', '10.0.0.2', '2025-01-03T00:00:00Z') RETURNING id, event_id`, f.TenantID()).Scan(&anonymous, &anonymousID))
	repo := persistence.NewBusinessEventRepository()

	e, err := repo.Get(f.Ctx, withUserID)
	require.NoError(t, err)
	require.Equal(t, history.LegacyActionLogType, e.Type)
	require.Equal(t, businessevent.CategoryLegacy, e.Category)
	require.Nil(t, e.Actor, "no snapshot is invented for a legacy user")
	require.NotNil(t, e.Legacy)
	require.True(t, e.Legacy.Incomplete)
	require.Equal(t, strconv.FormatUint(uint64(known.ID()), 10), e.Legacy.UserID)
	require.Equal(t, strconv.Itoa(withUser), e.Legacy.SourceID)
	require.JSONEq(t, `{"method":"POST","path":"/finance/payments/5","before":{"amount":10},"after":{"amount":12},"user_agent":"Mozilla","ip":"10.0.0.1"}`, string(e.Legacy.Payload))
	require.Equal(t, "2025-01-02T03:04:05Z", e.OccurredAt.UTC().Format("2006-01-02T15:04:05Z07:00"))
	require.Empty(t, e.Subjects)

	e, err = repo.Get(f.Ctx, anonymousID)
	require.NoError(t, err)
	require.Nil(t, e.Actor)
	require.Empty(t, e.Legacy.UserID)
	require.Empty(t, e.Channel)
	require.Empty(t, e.Outcome)
	require.Equal(t, strconv.Itoa(anonymous), e.Legacy.SourceID)
}

func TestBusinessEventRepository_LegacyRequestLogRequiresItsColumns(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	_, err := f.Tx.Exec(f.Ctx, `INSERT INTO action_logs (tenant_id, method, path) VALUES ($1, 'GET', '/')`, f.TenantID())
	require.ErrorContains(t, err, "action_logs_legacy_request", "a request-log row without user agent and IP is still rejected")
}

func TestBusinessEventRepository_BulkRedeliveryDoesNotDuplicate(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	actor := createUser(t, f.Ctx, "Bulk", "Operator", "", "bulk-operator@example.com")
	w := newWriter(t, employeeV1())
	ctx := asUser(f.Ctx, actor, "import-2026-10")
	deliver := func() []uuid.UUID {
		ids := make([]uuid.UUID, 0, 3)
		for _, employee := range []string{"1", "2", "3"} {
			id, err := w.Record(ctx, employeeUpdate(employee, positionChange("Intern", "Engineer")))
			require.NoError(t, err)
			ids = append(ids, id)
		}
		return ids
	}

	first := deliver()
	second := deliver()
	require.Equal(t, first, second)
	require.Len(t, map[uuid.UUID]bool{first[0]: true, first[1]: true, first[2]: true}, 3)

	_, err := w.Record(ctx, employeeUpdate("2", positionChange("Intern", "Manager")))
	require.ErrorIs(t, err, businessevent.ErrConflict)

	events, err := persistence.NewBusinessEventRepository().ListByOperation(f.Ctx, "import-2026-10")
	require.NoError(t, err)
	require.Len(t, events, 3)
	subjects := make([]string, 0, len(events))
	for _, e := range events {
		require.Len(t, e.Subjects, 1)
		subjects = append(subjects, e.Subjects[0].ID)
		require.JSONEq(t, `"Engineer"`, string(e.Changes[0].Fields[0].After.Value))
	}
	require.ElementsMatch(t, []string{"1", "2", "3"}, subjects)
}

func TestBusinessEventRepository_HiddenFieldsDoNotLeak(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	actor := createUser(t, f.Ctx, "Hidden", "Fields", "", "hidden-fields@example.com")
	registry := history.NewRegistry()
	require.NoError(t, registry.Register(employeeV1()))
	repo := persistence.NewBusinessEventRepository()

	id, err := history.NewWriter(registry, repo).Record(asUser(f.Ctx, actor, "op-salary"), employeeUpdate("5",
		fielddiff.Diff{Field: "salary", Before: value(`{"amount":"1000.00","currency":"UZS"}`), After: value(`{"amount":"1500.00","currency":"UZS"}`)},
		fielddiff.Diff{Field: "pin", Before: value(`"1111"`), After: value(`"9999"`)},
	))
	require.NoError(t, err)

	var stored string
	require.NoError(t, f.Tx.QueryRow(f.Ctx, `SELECT changes::text || actor_snapshot::text FROM action_logs WHERE event_id = $1`, id).Scan(&stored))
	require.NotContains(t, stored, "9999")
	require.NotContains(t, stored, "1111")
	require.NotContains(t, stored, "hidden-fields@example.com")

	e, err := repo.Get(f.Ctx, id)
	require.NoError(t, err)
	tests := []struct {
		name       string
		can        bool
		wantSalary fielddiff.FieldValue
	}{
		{"reader without permission", false, fielddiff.Redacted()},
		{"reader with permission", true, value(`{"amount":"1500.00","currency":"UZS"}`)},
	}
	for _, tt := range tests {
		visible := registry.ForReader(e, func(permission.Permission) bool { return tt.can })
		fields := map[string]fielddiff.Diff{}
		for _, d := range visible.Changes[0].Fields {
			fields[d.Field] = d
		}
		require.Equal(t, fielddiff.Redacted(), fields["pin"].After, tt.name)
		require.Equal(t, tt.wantSalary.State, fields["salary"].After.State, tt.name)
		if tt.can {
			require.JSONEq(t, string(tt.wantSalary.Value), string(fields["salary"].After.Value), tt.name)
		}
	}
}

func TestBusinessEventRepository_OldSchemaVersionStaysReadable(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	actor := createUser(t, f.Ctx, "Schema", "Versions", "", "schema-versions@example.com")
	repo := persistence.NewBusinessEventRepository()
	v1ID, err := newWriter(t, employeeV1()).Record(asUser(f.Ctx, actor, "op-v1"), employeeUpdate("8", positionChange("Clerk", "Analyst")))
	require.NoError(t, err)

	v2 := employeeDefinition(2, history.Field{Name: "grade", LabelKey: "Employees.Fields.Grade", Type: history.ValueInteger})
	registry := history.NewRegistry()
	require.NoError(t, registry.Register(employeeV1(), v2))
	in := employeeUpdate("8", fielddiff.Diff{Field: "grade", Before: value(`3`), After: value(`4`)})
	in.Version = 2
	v2ID, err := history.NewWriter(registry, repo).Record(asUser(f.Ctx, actor, "op-v2"), in)
	require.NoError(t, err)

	old, err := repo.Get(f.Ctx, v1ID)
	require.NoError(t, err)
	require.Equal(t, 1, old.SchemaVersion)
	visible := registry.ForReader(old, func(permission.Permission) bool { return true })
	require.JSONEq(t, `"Analyst"`, string(visible.Changes[0].Fields[0].After.Value))

	current, err := repo.Get(f.Ctx, v2ID)
	require.NoError(t, err)
	require.Equal(t, 2, current.SchemaVersion)

	onlyV2 := history.NewRegistry()
	require.NoError(t, onlyV2.Register(v2))
	visible = onlyV2.ForReader(old, func(permission.Permission) bool { return true })
	require.Equal(t, fielddiff.Redacted(), visible.Changes[0].Fields[0].After, "values without a definition are hidden, the event still reads")
}

func TestBusinessEventRepository_FailedRecordRollsBackBusinessChange(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	actor := createUser(t, f.Ctx, "Atomic", "Writer", "", "atomic-writer@example.com")
	users := corepersistence.NewUserRepository(corepersistence.NewUploadRepository())
	_, err := f.Tx.Exec(f.Ctx, `CREATE FUNCTION fail_history() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'history storage unavailable'; END; $$ LANGUAGE plpgsql`)
	require.NoError(t, err)
	_, err = f.Tx.Exec(f.Ctx, `CREATE TRIGGER fail_history BEFORE INSERT ON action_logs FOR EACH ROW EXECUTE FUNCTION fail_history()`)
	require.NoError(t, err)

	business, err := f.Tx.Begin(f.Ctx)
	require.NoError(t, err)
	ctx := composables.WithTx(asUser(f.Ctx, actor, "op-atomic"), business)
	created := createUser(t, ctx, "Rolled", "Back", "", "rolled-back@example.com")
	_, err = newWriter(t, employeeV1()).Record(ctx, employeeUpdate("1", positionChange("A", "B")))
	require.ErrorIs(t, err, history.ErrPersistenceFailed)
	require.NoError(t, business.Rollback(f.Ctx))

	_, err = users.GetByID(f.Ctx, created.ID())
	require.Error(t, err, "the business change is rolled back with the event")
	var events int
	require.NoError(t, f.Tx.QueryRow(f.Ctx, `SELECT count(*) FROM action_logs WHERE event_type IS NOT NULL`).Scan(&events))
	require.Zero(t, events)
}

func TestBusinessEventRepository_AppendOnly(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	actor := createUser(t, f.Ctx, "Append", "Only", "", "append-only@example.com")
	id, err := newWriter(t, employeeV1()).Record(asUser(f.Ctx, actor, "op-append"), employeeUpdate("1", positionChange("A", "B")))
	require.NoError(t, err)

	var legacyID uuid.UUID
	require.NoError(t, f.Tx.QueryRow(f.Ctx, `INSERT INTO action_logs (tenant_id, method, path, user_agent, ip) VALUES ($1, 'GET', '/', 'curl', '10.0.0.1') RETURNING event_id`,
		f.TenantID()).Scan(&legacyID))
	tests := []struct {
		name   string
		sql    string
		target uuid.UUID
	}{
		{"update", `UPDATE action_logs SET event_type = 'hrm.employee.deleted' WHERE event_id = $1`, id},
		{"delete", `DELETE FROM action_logs WHERE event_id = $1`, id},
		{"turn a legacy record into an event", `UPDATE action_logs SET event_type = 'hrm.employee.deleted' WHERE event_id = $1`, legacyID},
	}
	for _, tt := range tests {
		sp, err := f.Tx.Begin(f.Ctx)
		require.NoError(t, err)
		_, err = sp.Exec(f.Ctx, tt.sql, tt.target)
		require.ErrorContains(t, err, "append-only", tt.name)
		require.NoError(t, sp.Rollback(f.Ctx))
	}
	_, err = f.Tx.Exec(f.Ctx, `UPDATE action_logs SET path = '/home' WHERE event_id = $1`, legacyID)
	require.NoError(t, err, "legacy request-log records keep their previous behavior")
	_, err = f.Tx.Exec(f.Ctx, `DELETE FROM action_logs WHERE event_id = $1`, legacyID)
	require.NoError(t, err)

	other := uuid.New()
	_, err = f.Tx.Exec(f.Ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2)`, other, "history-"+other.String())
	require.NoError(t, err)
	otherCtx := composables.WithTenantID(f.Ctx, other)
	_, err = newWriter(t, employeeV1()).Record(history.WithExecution(otherCtx, history.Execution{
		Channel: businessevent.ChannelJob, OperationID: "op-tenant", Actor: ptr(businessevent.SystemActor("scheduler", "Scheduler")),
	}), employeeUpdate("1", positionChange("A", "B")))
	require.NoError(t, err)
	_, err = f.Tx.Exec(f.Ctx, `DELETE FROM tenants WHERE id = $1`, other)
	require.NoError(t, err, "removing a tenant is the explicit way its history goes away")
	var left int
	require.NoError(t, f.Tx.QueryRow(f.Ctx, `SELECT count(*) FROM action_logs WHERE tenant_id = $1`, other).Scan(&left))
	require.Zero(t, left)
	_, err = persistence.NewBusinessEventRepository().Get(f.Ctx, id)
	require.NoError(t, err)
}

func TestBusinessEventRepository_AppendRequiresBusinessTransaction(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	actor := createUser(t, f.Ctx, "No", "Transaction", "", "no-transaction@example.com")
	ctx := composables.WithPool(asUser(context.Background(), actor, "op-no-tx"), f.Pool)
	ctx = composables.WithTenantID(ctx, f.TenantID())
	_, err := newWriter(t, employeeV1()).Record(ctx, employeeUpdate("1", positionChange("A", "B")))
	require.ErrorIs(t, err, history.ErrPersistenceFailed)
}

func TestBusinessEventRepository_AppendRejectsForeignTenant(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	err := persistence.NewBusinessEventRepository().Append(f.Ctx, businessevent.Event{
		ID: uuid.New(), TenantID: uuid.New(), Actor: ptr(businessevent.SystemActor("scheduler", "Scheduler")),
	})
	require.ErrorContains(t, err, "does not match the request tenant")
	var count int
	require.NoError(t, f.Tx.QueryRow(f.Ctx, `SELECT count(*) FROM action_logs WHERE event_type IS NOT NULL`).Scan(&count))
	require.Zero(t, count)
}

func ptr[T any](v T) *T {
	return &v
}

func TestLoggingComponent_ProvidesHistory(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	registry, err := composition.Resolve[*history.Registry](f.Container)
	require.NoError(t, err)
	_, ok := registry.Get(history.LegacyActionLogType, 1)
	require.True(t, ok)
	writer, err := composition.Resolve[history.Writer](f.Container)
	require.NoError(t, err)
	require.NotNil(t, writer)
}
