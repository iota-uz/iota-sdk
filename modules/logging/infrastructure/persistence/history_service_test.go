package persistence_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/logging/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/logging/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

var contractRead = permission.MustCreate(uuid.MustParse("0b1c8d4e-2f3a-4b5c-9d6e-7f8091a2b3c4"), "Contracts.Read", "contracts", permission.ActionRead, permission.ModifierAll)

var historyStart = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

func contractSigned() history.Definition {
	return history.Definition{
		Type:           "hrm.contract.signed",
		Version:        1,
		Category:       businessevent.CategoryBusiness,
		TitleKey:       "Contracts.History.Signed.Title",
		DescriptionKey: "Contracts.History.Signed.Description",
		Resources:      []history.Resource{{Name: "contract", ReadPermission: &contractRead}},
	}
}

type historyFixture struct {
	ctx     context.Context
	service *history.Service
	actor   string
}

// newHistoryFixture records five employee updates an hour apart (employees 1..5, newest last),
// one contract signing and one legacy action log record.
func newHistoryFixture(t *testing.T) historyFixture {
	t.Helper()
	f := setupTest(t)
	registry := history.NewRegistry()
	require.NoError(t, registry.Register(employeeV1(), contractSigned(), history.LegacyActionLogDefinition()))
	repo := persistence.NewBusinessEventRepository()
	writer := history.NewWriter(registry, repo)
	actor := createUser(t, f.Ctx, "Ali", "Valiyev", "", "history-reader-actor@example.com")
	for i := 1; i <= 5; i++ {
		in := employeeUpdate(strconv.Itoa(i), positionChange("Clerk", "Analyst"),
			fielddiff.Diff{Field: "salary", Before: value(`{"amount":"100","currency":"UZS"}`), After: value(`{"amount":"200","currency":"UZS"}`)})
		in.OccurredAt = historyStart.Add(time.Duration(i) * time.Hour)
		_, err := writer.Record(asUser(f.Ctx, actor, "op-"+strconv.Itoa(i)), in)
		require.NoError(t, err)
	}
	_, err := writer.Record(asUser(f.Ctx, actor, "op-contract"), history.Input{
		Type: "hrm.contract.signed", Version: 1, Outcome: businessevent.OutcomeSucceeded, OccurredAt: historyStart,
		Subjects: []businessevent.Subject{{Resource: "contract", ID: "77"}},
	})
	require.NoError(t, err)
	_, err = f.Tx.Exec(f.Ctx, `INSERT INTO action_logs (tenant_id, method, path, user_id, user_agent, ip, created_at)
		VALUES ($1, 'POST', '/finance/payments', $2, 'test', '127.0.0.1', $3)`, f.TenantID(), actor.ID(), historyStart.Add(-time.Hour))
	require.NoError(t, err)
	require.NoError(t, corepersistence.NewUserRepository(corepersistence.NewUploadRepository()).Delete(f.Ctx, actor.ID()))
	return historyFixture{
		ctx:     f.Ctx,
		service: history.NewService(registry, repo),
		actor:   strconv.FormatUint(uint64(actor.ID()), 10),
	}
}

func (h historyFixture) as(perms ...permission.Permission) context.Context {
	return composables.WithUser(h.ctx, itf.User(perms...))
}

func all() history.Scope {
	return history.Scope{Kind: history.ScopeAll}
}

func eventTypes(result history.Result) []string {
	types := make([]string, len(result.Items))
	for i, item := range result.Items {
		types[i] = item.EventType
	}
	return types
}

func TestHistoryService_Filters(t *testing.T) {
	t.Parallel()
	h := newHistoryFixture(t)
	reader := h.as(permissions.ViewLogs, contractRead)

	tests := []struct {
		name  string
		query history.Query
		count int
	}{
		{"everything", history.Query{Scope: all()}, 7},
		{"resource", history.Query{Scope: history.Scope{Kind: history.ScopeResource, Resource: "employee"}}, 5},
		{"entity", history.Query{Scope: history.Scope{Kind: history.ScopeEntity, Resource: "employee", ID: "3"}}, 1},
		{"event type", history.Query{Scope: all(), EventTypes: []string{"hrm.contract.signed"}}, 1},
		{"deleted actor, legacy record loses its user", history.Query{Scope: all(), ActorID: h.actor}, 6},
		{"channel", history.Query{Scope: all(), Channels: []string{"ui"}}, 6},
		{"legacy channel", history.Query{Scope: all(), Channels: []string{history.LegacyChannel}}, 1},
		{"period from inclusive, to exclusive", history.Query{Scope: all(),
			From: historyStart.Add(2 * time.Hour).Format(time.RFC3339), To: historyStart.Add(4 * time.Hour).Format(time.RFC3339)}, 2},
		{"empty selection", history.Query{Scope: all(), Outcomes: []businessevent.Outcome{businessevent.OutcomeFailed}}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := h.service.History(reader, tt.query)
			require.NoError(t, err)
			require.Len(t, result.Items, tt.count)
			require.Nil(t, result.NextCursor)
			require.False(t, result.Truncated)
			require.NotNil(t, result.Items)
		})
	}
}

func TestHistoryService_CursorWalksTimelineAndIsBoundToFilters(t *testing.T) {
	t.Parallel()
	h := newHistoryFixture(t)
	reader := h.as(permissions.ViewLogs, contractRead)

	query := history.Query{Scope: all(), Limit: 3}
	var seen []string
	var last time.Time
	for page := 0; ; page++ {
		result, err := h.service.History(reader, query)
		require.NoError(t, err)
		for _, item := range result.Items {
			if last.IsZero() {
				last = item.OccurredAt
			}
			require.False(t, item.OccurredAt.After(last), "items are newest first")
			last = item.OccurredAt
			seen = append(seen, item.EventID)
		}
		if result.NextCursor == nil {
			break
		}
		require.Len(t, result.Items, 3)
		query.Cursor = *result.NextCursor
		require.Less(t, page, 5)
	}
	require.Len(t, seen, 7)
	require.Len(t, map[string]bool{seen[0]: true, seen[1]: true, seen[2]: true, seen[3]: true, seen[4]: true, seen[5]: true, seen[6]: true}, 7)

	first, err := h.service.History(reader, history.Query{Scope: all(), Limit: 2})
	require.NoError(t, err)
	require.NotNil(t, first.NextCursor)
	for name, q := range map[string]history.Query{
		"other filters": {Scope: all(), Channels: []string{"ui"}, Cursor: *first.NextCursor},
		"other scope":   {Scope: history.Scope{Kind: history.ScopeResource, Resource: "employee"}, Cursor: *first.NextCursor},
		"other user":    {Scope: all(), Cursor: *first.NextCursor},
		"forged":        {Scope: all(), Cursor: "not-a-cursor"},
	} {
		ctx := reader
		if name == "other user" {
			email, err := internet.NewEmail("other-reader@example.com")
			require.NoError(t, err)
			ctx = composables.WithUser(h.ctx, user.New("Other", "Reader", email, user.UILanguageEN,
				user.WithID(2), user.WithRoles(itf.User(permissions.ViewLogs, contractRead).Roles())))
		}
		_, err := h.service.History(ctx, q)
		require.ErrorIs(t, err, history.ErrInvalidInput, name)
	}
	next, err := h.service.History(reader, history.Query{Scope: all(), Limit: 5, Cursor: *first.NextCursor})
	require.NoError(t, err, "the page size may change between pages")
	require.Len(t, next.Items, 5)
}

func TestHistoryService_RejectsInvalidInput(t *testing.T) {
	t.Parallel()
	h := newHistoryFixture(t)
	reader := h.as(permissions.ViewLogs)
	for name, q := range map[string]history.Query{
		"unknown scope":       {Scope: history.Scope{Kind: "table"}},
		"unregistered":        {Scope: history.Scope{Kind: history.ScopeResource, Resource: "users"}},
		"entity without id":   {Scope: history.Scope{Kind: history.ScopeEntity, Resource: "employee"}},
		"empty event types":   {Scope: all(), EventTypes: []string{}},
		"unknown event type":  {Scope: all(), EventTypes: []string{"hrm.employee.fired"}},
		"unknown channel":     {Scope: all(), Channels: []string{"email"}},
		"unknown outcome":     {Scope: all(), Outcomes: []businessevent.Outcome{"applied"}},
		"from not before to":  {Scope: all(), From: "2026-10-02T00:00:00Z", To: "2026-10-01T00:00:00Z"},
		"time is not rfc3339": {Scope: all(), From: "yesterday"},
		"limit above max":     {Scope: all(), Limit: history.MaxLimit + 1},
	} {
		_, err := h.service.History(reader, q)
		require.ErrorIs(t, err, history.ErrInvalidInput, name)
	}
}

func TestHistoryService_AppliesPermissionsBeforePagination(t *testing.T) {
	t.Parallel()
	h := newHistoryFixture(t)

	_, err := h.service.History(h.as(), history.Query{Scope: all()})
	require.ErrorIs(t, err, history.ErrForbidden, "viewing the log needs its own permission")

	reader := h.as(permissions.ViewLogs)
	_, err = h.service.History(reader, history.Query{Scope: history.Scope{Kind: history.ScopeResource, Resource: "contract"}})
	require.ErrorIs(t, err, history.ErrForbidden)

	result, err := h.service.History(reader, history.Query{Scope: all(), Limit: 6})
	require.NoError(t, err)
	require.NotContains(t, eventTypes(result), "hrm.contract.signed")
	require.Len(t, result.Items, 6, "hidden events do not take places on the page")
	require.Nil(t, result.NextCursor)

	for _, item := range result.Items {
		for _, c := range item.Changes {
			for _, d := range c.Fields {
				if d.Field == "salary" {
					require.Equal(t, fielddiff.StateRedacted, d.Before.State)
					require.Equal(t, fielddiff.StateRedacted, d.After.State)
				}
			}
		}
	}
}

func TestHistoryService_DeletedActorAndLegacyRecordStayReadable(t *testing.T) {
	t.Parallel()
	h := newHistoryFixture(t)
	result, err := h.service.History(h.as(permissions.ViewLogs), history.Query{Scope: all(), ActorID: h.actor})
	require.NoError(t, err)

	newest := result.Items[0]
	require.Equal(t, "hrm.employee.updated", newest.EventType)
	require.NotNil(t, newest.Actor)
	require.Equal(t, "Ali", newest.Actor.FirstName)
	require.Equal(t, "Valiyev", newest.Actor.LastName)
	require.Equal(t, "ui", newest.Channel)
	require.Equal(t, "op-5", *newest.OperationID)
	require.Equal(t, []history.Subject{{Resource: "employee", ID: "5", Label: "employee #5"}}, newest.Subjects)
	require.False(t, newest.LegacyIncomplete)

	result, err = h.service.History(h.as(permissions.ViewLogs), history.Query{Scope: all()})
	require.NoError(t, err)
	legacy := result.Items[len(result.Items)-1]
	require.Equal(t, history.LegacyActionLogType, legacy.EventType)
	require.True(t, legacy.LegacyIncomplete)
	require.Nil(t, legacy.Actor)
	require.Nil(t, legacy.OperationID)
	require.Equal(t, history.LegacyChannel, legacy.Channel)
	require.Empty(t, legacy.Subjects)
	require.Equal(t, historyStart.Add(-time.Hour), legacy.OccurredAt.UTC())
}
