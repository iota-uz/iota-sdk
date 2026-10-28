package services_test

import (
	"context"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/general"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/crm"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	crmpermissions "github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/modules/logging"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

var allClientFields = []string{
	"address", "comments", "date_of_birth", "email", "first_name", "gender", "last_name", "middle_name", "passport", "phone", "pin",
}

type crmHistoryFixture struct {
	env     *itf.TestEnvironment
	clients *services.ClientService
	chats   *services.ChatService
	events  businessevent.Repository
}

func setupCRMHistory(t *testing.T) crmHistoryFixture {
	t.Helper()
	env := itf.Setup(t, itf.WithComponents(
		logging.NewComponent(),
		core.NewComponent(&core.ModuleOptions{PermissionSchema: defaults.PermissionSchema()}),
		crm.NewComponent(),
	), itf.WithUser(itf.User(crmpermissions.ClientRead)))
	events, err := composition.Resolve[businessevent.Repository](env.Container)
	require.NoError(t, err)
	return crmHistoryFixture{
		env:     env,
		clients: itf.GetService[services.ClientService](env),
		chats:   itf.GetService[services.ChatService](env),
		events:  events,
	}
}

func (f crmHistoryFixture) newClient(t *testing.T, ctx context.Context, number string) client.Client {
	t.Helper()
	p, err := phone.NewFromE164(number)
	require.NoError(t, err)
	birthDate := time.Date(1990, 5, 17, 0, 0, 0, 0, time.UTC)
	c, err := client.New("Aziz",
		client.WithTenantID(f.env.Tenant.ID),
		client.WithLastName("Karimov"),
		client.WithPhone(p),
		client.WithGender(general.Male),
		client.WithDateOfBirth(&birthDate),
		client.WithPassport(passport.New("AB", "1234567")),
	)
	require.NoError(t, err)
	require.NoError(t, f.clients.Create(ctx, c))
	created, err := f.clients.GetByPhone(f.env.Ctx, p.Value())
	require.NoError(t, err)
	return created
}

func (f crmHistoryFixture) operationEvents(t *testing.T, ctx context.Context) []businessevent.Event {
	t.Helper()
	exec, ok := history.UseExecution(ctx)
	require.True(t, ok)
	events, err := f.events.ListByOperation(f.env.Ctx, exec.OperationID)
	require.NoError(t, err)
	return events
}

func clientID(c client.Client) string {
	return strconv.FormatUint(uint64(c.ID()), 10)
}

func TestCRMHistory_RecordsServiceMutations(t *testing.T) {
	f := setupCRMHistory(t)

	tests := []struct {
		name      string
		act       func(t *testing.T, ctx context.Context) string
		eventType string
		fields    []string
	}{
		{
			name: "client created",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				return clientID(f.newClient(t, ctx, "998901110001"))
			},
			eventType: services.ClientCreated,
			fields:    allClientFields,
		},
		{
			name: "client updated",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				c := f.newClient(t, f.env.Ctx, "998901110002")
				require.NoError(t, f.clients.Update(ctx, c.SetName("Aziza", "Karimova", "").SetComments("VIP")))
				return clientID(c)
			},
			eventType: services.ClientUpdated,
			fields:    []string{"comments", "first_name", "last_name"},
		},
		{
			name: "client deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				c := f.newClient(t, f.env.Ctx, "998901110003")
				_, err := f.clients.Delete(ctx, c.ID())
				require.NoError(t, err)
				return clientID(c)
			},
			eventType: services.ClientDeleted,
			fields:    allClientFields,
		},
		{
			name: "guest client created from an incoming chat number",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				ctx = context.WithValue(ctx, constants.LoggerKey, logrus.NewEntry(logrus.New()))
				c, err := f.chats.CreateOrGetClientByPhone(ctx, "+998901110004")
				require.NoError(t, err)
				return clientID(c)
			},
			eventType: services.ClientCreated,
			fields:    allClientFields,
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
			require.Equal(t, []businessevent.Subject{{Resource: services.ClientResource, ID: id}}, e.Subjects)
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

func TestCRMHistory_SensitiveFieldsKeepValues(t *testing.T) {
	f := setupCRMHistory(t)
	ctx := itf.TestExecution(f.env.Ctx)

	f.newClient(t, ctx, "998901110005")

	events := f.operationEvents(t, ctx)
	require.Len(t, events, 1)
	values := map[string]string{}
	for _, d := range events[0].Changes[0].Fields {
		values[d.Field] = string(d.After.Value)
	}
	require.JSONEq(t, `"AB1234567"`, values["passport"])
	require.JSONEq(t, `"1990-05-17"`, values["date_of_birth"])
}

func TestCRMHistory_UnchangedUpdateRecordsNothing(t *testing.T) {
	f := setupCRMHistory(t)
	c := f.newClient(t, f.env.Ctx, "998901110006")
	ctx := itf.TestExecution(f.env.Ctx)

	require.NoError(t, f.clients.Update(ctx, c))

	require.Empty(t, f.operationEvents(t, ctx))
}

func TestCRMHistory_RollbackLeavesNoEvents(t *testing.T) {
	f := setupCRMHistory(t)

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
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			savepoint, err := f.env.Tx.Begin(f.env.Ctx)
			require.NoError(t, err)
			operation := itf.TestExecution(composables.WithTx(f.env.Ctx, savepoint))
			p, err := phone.NewFromE164("99890222000" + strconv.Itoa(i))
			require.NoError(t, err)
			c, err := client.New("Rolled back", client.WithTenantID(f.env.Tenant.ID), client.WithPhone(p))
			require.NoError(t, err)

			err = f.clients.Create(tt.ctx(operation, savepoint), c)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				require.Len(t, f.operationEvents(t, operation), 1)
			}
			require.NoError(t, savepoint.Rollback(f.env.Ctx))

			require.Empty(t, f.operationEvents(t, operation))
			_, err = f.clients.GetByPhone(f.env.Ctx, p.Value())
			require.ErrorIs(t, err, persistence.ErrClientNotFound)
		})
	}
}
