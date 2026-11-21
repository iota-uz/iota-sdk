package services_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

type outboxRow = itf.OutboxRow

func TestUserService_Create_PublishesUserCreatedEventInTheCreatingTransaction(t *testing.T) {
	e := newEventEnv(t)
	ids := []string{services.UserCreatedEventID}
	got := e.commitsAndRollsBack(t, ids, map[string]int{services.UserCreatedEventID: 1},
		itf.Domain{SQL: "SELECT count(*)::text FROM users WHERE tenant_id = $1", Args: []any{e.tenantID}},
		func(ctx context.Context, suffix string) error {
			email := internet.MustParseEmail("event-user-" + suffix + "@example.com")
			_, err := e.users.Create(ctx, user.New("Event", "User", email, user.UILanguageEN, user.WithTenantID(e.tenantID)))
			return err
		})
	last := got[services.UserCreatedEventID][0]
	require.Equal(t, services.UserCreatedEventID, last.Event)
	require.Equal(t, "core.user", last.SubjectEntity)
	p := payloadOf[services.UserCreatedPayload](t, last)
	require.Equal(t, fmt.Sprint(p.UserID), last.SubjectKey)
	require.JSONEq(t, fmt.Sprintf(`{"userId":"%d","email":"event-user-committed@example.com","firstName":"Event","lastName":"User","userType":"user"}`, p.UserID), string(last.Payload))

	// A rejected creation (duplicate email) publishes nothing.
	before := e.eventCounts(t, e.ctx(), services.UserCreatedEventID)
	_, err := e.users.Create(e.ctx(), user.New("Dup", "User", internet.MustParseEmail("event-user-committed@example.com"), user.UILanguageEN, user.WithTenantID(e.tenantID)))
	require.Error(t, err)
	require.Equal(t, before, e.eventCounts(t, e.ctx(), services.UserCreatedEventID))
}

func TestUserCreatedEvent_IsInTheNativeCatalogWithAVersionedPayload(t *testing.T) {
	set, err := native.Compose(services.Natives)
	require.NoError(t, err)
	def, ok := set.Event(services.UserCreatedEventID)
	require.True(t, ok, "the core module contributes the event to the native set")
	res := def.Resource()
	require.Equal(t, manifest.ResEvent, res.Kind)
	require.Equal(t, "core.user", res.Subject)
	require.NotNil(t, res.Payload)
	require.NotEmpty(t, res.ContractHash)
	require.NoError(t, manifest.ValidateValue(*res.Payload, json.RawMessage(`{"userId":"1","email":"a@b.c","firstName":"A","lastName":"B","userType":"user"}`)))
	require.Error(t, manifest.ValidateValue(*res.Payload, json.RawMessage(`{"userId":1}`)), "int64 is a decimal string on the wire")
}
