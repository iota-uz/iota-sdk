package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/crm"
	crmpermissions "github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/modules/logging"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func clientEvents(t *testing.T, env *itf.TestEnvironment, event string) []json.RawMessage {
	t.Helper()
	rows, err := env.Pool.Query(context.Background(), `SELECT payload FROM extruntime.outbox_events WHERE tenant_id = $1 AND event = $2 ORDER BY seq`, env.TenantID(), event)
	require.NoError(t, err)
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var p json.RawMessage
		require.NoError(t, rows.Scan(&p))
		out = append(out, p)
	}
	require.NoError(t, rows.Err())
	return out
}

// saveThenFail writes through crm.operations.upsertClient and then fails, so
// its transaction rolls back after the event was enqueued.
var saveThenFail = native.Command("test.commands.saveThenFail", native.Spec{}, func(native.Deps) (native.CommandFunc[services.SaveClientInput, services.SaveClientOutput], error) {
	return func(ctx context.Context, c native.CommandContext, in services.SaveClientInput) (services.SaveClientOutput, error) {
		return services.SaveClientOutput{}, c.Transaction(ctx, func(ctx context.Context, tx native.Tx) error {
			if _, err := services.UpsertClient.Call(ctx, tx, in); err != nil {
				return err
			}
			return errors.New("fail after the write")
		})
	}, nil
})

func dispatch[O any](t *testing.T, ctx context.Context, d native.Dispatcher, contract string, in any) (O, error) {
	t.Helper()
	var out O
	raw, err := json.Marshal(in)
	require.NoError(t, err)
	res, err := d.Dispatch(ctx, contract, raw)
	if err != nil {
		return out, err
	}
	require.NoError(t, json.Unmarshal(res, &out))
	return out, nil
}

// The client events commit with the client they report and roll back with it.
func TestClientEvents_CommitAndRollBackWithTheClient(t *testing.T) {
	env := itf.Setup(t, itf.WithComponents(
		logging.NewComponent(),
		core.NewComponent(&core.ModuleOptions{PermissionSchema: defaults.PermissionSchema()}),
		crm.NewComponent(),
	), itf.WithUser(itf.User(crmpermissions.ClientCreate, crmpermissions.ClientUpdate, crmpermissions.ClientRead)))
	// The upsert owns its transaction, so the fixture is committed first.
	require.NoError(t, env.Tx.Commit(env.Ctx))
	ctx := composables.WithPool(context.Background(), env.Pool)
	ctx = composables.WithTenantID(ctx, env.TenantID())
	ctx = composables.WithUser(ctx, env.User)
	ctx = itf.TestExecution(ctx)

	set := services.Contracts()
	require.NoError(t, set.Add(saveThenFail))
	d := native.NewDirect(native.Bind(set, native.StaticDeps(itf.GetService[services.ClientService](env))), native.WithAuthorizer(native.UserAuthorizer))
	str := func(s string) *string { return &s }
	createdBefore := len(clientEvents(t, env, services.ClientCreatedEventID))
	updatedBefore := len(clientEvents(t, env, services.ClientUpdatedEventID))

	// A rolled back creation leaves no event.
	_, err := dispatch[services.SaveClientOutput](t, ctx, d, saveThenFail.ID(), services.SaveClientInput{FirstName: str("Rolled"), Phone: str("+998901112233")})
	require.ErrorContains(t, err, "fail after the write")
	require.Len(t, clientEvents(t, env, services.ClientCreatedEventID), createdBefore)

	// A committed creation publishes crm.client.created.v1.
	out, err := dispatch[services.SaveClientOutput](t, ctx, d, services.SaveClient.ID(), services.SaveClientInput{FirstName: str("Aziz"), Phone: str("+998901112244"), Email: str("aziz@example.com")})
	require.NoError(t, err)
	require.True(t, out.Created)
	created := clientEvents(t, env, services.ClientCreatedEventID)
	require.Len(t, created, createdBefore+1)
	var payload services.ClientCreatedPayload
	require.NoError(t, json.Unmarshal(created[len(created)-1], &payload))
	require.Equal(t, "Aziz", payload.FirstName)

	// A rolled back update leaves no event; a committed one publishes
	// crm.client.updated.v1.
	ref := out.Client
	_, err = dispatch[services.SaveClientOutput](t, ctx, d, saveThenFail.ID(), services.SaveClientInput{Client: &ref, FirstName: str("Rolled")})
	require.ErrorContains(t, err, "fail after the write")
	require.Len(t, clientEvents(t, env, services.ClientUpdatedEventID), updatedBefore)
	_, err = dispatch[services.SaveClientOutput](t, ctx, d, services.SaveClient.ID(), services.SaveClientInput{Client: &ref, FirstName: str("Akmal")})
	require.NoError(t, err)
	updated := clientEvents(t, env, services.ClientUpdatedEventID)
	require.Len(t, updated, updatedBefore+1)
	var up services.ClientUpdatedPayload
	require.NoError(t, json.Unmarshal(updated[len(updated)-1], &up))
	require.Equal(t, "Akmal", up.FirstName)
	require.Equal(t, []string{"firstName"}, up.Changed)
}
