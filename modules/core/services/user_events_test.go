package services_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

type outboxRow struct {
	Event, SubjectEntity, SubjectKey string
	Initiator                        *int64
	Payload                          json.RawMessage
}

func userCreatedEvents(t *testing.T, ctx context.Context, tenantID uuid.UUID) []outboxRow {
	t.Helper()
	tx, err := composables.UseTx(ctx)
	require.NoError(t, err)
	rows, err := tx.Query(ctx, `
		SELECT event, subject_entity, subject_key, initiator_user_id, payload
		FROM extruntime.outbox_events WHERE tenant_id = $1 AND event = $2 ORDER BY seq`, tenantID, services.UserCreatedEventID)
	require.NoError(t, err)
	defer rows.Close()
	var out []outboxRow
	for rows.Next() {
		var r outboxRow
		require.NoError(t, rows.Scan(&r.Event, &r.SubjectEntity, &r.SubjectKey, &r.Initiator, &r.Payload))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func TestUserService_Create_PublishesUserCreatedEventInTheCreatingTransaction(t *testing.T) {
	f := setupTest(t)
	tenantID := f.TenantID()
	require.NoError(t, persistence.NewPermissionRepository().Save(f.Ctx, permissions.UserCreate))
	userRepository := persistence.NewUserRepository(persistence.NewUploadRepository())
	actorEmail, err := internet.NewEmail("event-admin@example.com")
	require.NoError(t, err)
	actor, err := userRepository.Create(f.Ctx, user.New("Event", "Admin", actorEmail, user.UILanguageEN,
		user.WithTenantID(tenantID), user.WithPermissions([]permission.Permission{permissions.UserCreate})))
	require.NoError(t, err)
	ctx := composables.WithUser(f.Ctx, actor)
	userService := itf.GetService[services.UserService](f)
	require.NotNil(t, userService)
	before := len(userCreatedEvents(t, ctx, tenantID))

	email, err := internet.NewEmail("event-user@example.com")
	require.NoError(t, err)
	created, err := userService.Create(ctx, user.New("Event", "User", email, user.UILanguageEN, user.WithTenantID(tenantID)))
	require.NoError(t, err)

	got := userCreatedEvents(t, ctx, tenantID)
	require.Len(t, got, before+1, "exactly one event per created user, visible in the creating transaction")
	last := got[len(got)-1]
	require.Equal(t, "core.user", last.SubjectEntity)
	require.Equal(t, fmt.Sprint(created.ID()), last.SubjectKey)
	require.NotNil(t, last.Initiator, "the acting user is the event initiator")
	require.EqualValues(t, actor.ID(), *last.Initiator)
	require.JSONEq(t, fmt.Sprintf(`{"userId":"%d","email":"event-user@example.com","firstName":"Event","lastName":"User","userType":"user"}`, created.ID()), string(last.Payload))

	// A rejected creation (duplicate email) publishes nothing.
	_, err = userService.Create(ctx, user.New("Dup", "User", email, user.UILanguageEN, user.WithTenantID(tenantID)))
	require.Error(t, err)
	require.Len(t, userCreatedEvents(t, ctx, tenantID), before+1)
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
