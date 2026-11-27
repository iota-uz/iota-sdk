package services_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
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

// The payload carries the created user's phone in E.164 when the user has
// one, and leaves the field out when the user has none.
func TestUserService_Create_PublishesThePhoneOfTheUser(t *testing.T) {
	e := newEventEnv(t)
	ctx := e.ctx()
	create := func(t *testing.T, opts ...user.Option) map[string]any {
		t.Helper()
		before := len(outboxRows(t, ctx, e.tenantID, services.UserCreatedEventID))
		email := internet.MustParseEmail("phone-" + uuid.NewString() + "@example.com")
		_, err := e.users.Create(ctx, user.New("Phone", "User", email, user.UILanguageEN, append([]user.Option{user.WithTenantID(e.tenantID)}, opts...)...))
		require.NoError(t, err)
		rows := outboxRows(t, ctx, e.tenantID, services.UserCreatedEventID)
		require.Len(t, rows, before+1)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(rows[len(rows)-1].Payload, &payload))
		return payload
	}

	t.Run("with a phone", func(t *testing.T) {
		p, err := phone.NewFromE164("+998901234567")
		require.NoError(t, err)
		payload := create(t, user.WithPhone(p))
		require.Equal(t, "+998901234567", payload["phone"])
	})

	t.Run("without a phone", func(t *testing.T) {
		payload := create(t)
		require.NotContains(t, payload, "phone", "a user without a phone publishes no phone field")
	})
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
	require.NoError(t, manifest.ValidateValue(*res.Payload, json.RawMessage(`{"userId":"1","email":"a@b.c","firstName":"A","lastName":"B","userType":"user","phone":"+998901234567"}`)), "phone is optional")
	require.Error(t, manifest.ValidateValue(*res.Payload, json.RawMessage(`{"userId":1}`)), "int64 is a decimal string on the wire")
	meta, ok := def.(native.EventMetadata)
	require.True(t, ok)
	require.Equal(t, map[string][]string{"email": {"User.Read"}, "phone": {"User.Read"}}, meta.Sensitive(), "the phone is personal data, hidden like the email")
}
