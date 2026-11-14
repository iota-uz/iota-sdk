package history_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	loghistory "github.com/iota-uz/iota-sdk/modules/logging/history"
	historypage "github.com/iota-uz/iota-sdk/modules/logging/presentation/templates/pages/history"
	"github.com/iota-uz/iota-sdk/pkg/filterq"
	"github.com/stretchr/testify/require"
)

var schema = filterq.Schema{Fields: []filterq.Field{
	{Key: historypage.FieldPeriod, Type: filterq.FieldTypeDate},
	{Key: historypage.FieldResource, Type: filterq.FieldTypeReference},
	{Key: historypage.FieldEventType, Type: filterq.FieldTypeReference},
	{Key: historypage.FieldChannel, Type: filterq.FieldTypeReference},
	{Key: historypage.FieldOutcome, Type: filterq.FieldTypeReference},
}}

func parse(t *testing.T, rawQuery string) historypage.Filters {
	t.Helper()
	values, err := url.ParseQuery(rawQuery)
	require.NoError(t, err)
	return historypage.ParseFilters(values, schema)
}

func tashkent(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Tashkent")
	require.NoError(t, err)
	return loc
}

var now = time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

func TestQuery_ListsSeveralResourcesAndActions(t *testing.T) {
	t.Parallel()
	f := parse(t, "fb=1&f=resource:is:finance.expense,finance.payment&f=event_type:is:finance.expense.created,finance.payment.created&f=channel:is:ui,mcp&f=outcome:is:failed,denied")

	q := f.Query(nil, now)
	require.Equal(t, loghistory.Scope{Kind: loghistory.ScopeAll}, q.Scope)
	require.Equal(t, []string{"finance.expense", "finance.payment"}, q.Resources)
	require.Equal(t, []string{"finance.expense.created", "finance.payment.created"}, q.EventTypes)
	require.Equal(t, []string{"ui", "mcp"}, q.Channels)
	require.Equal(t, []businessevent.Outcome{businessevent.OutcomeFailed, businessevent.OutcomeDenied}, q.Outcomes)
}

func TestQuery_PeriodDaysFollowTheApplicationTimeZone(t *testing.T) {
	t.Parallel()
	loc := tashkent(t)
	tests := []struct {
		name     string
		query    string
		loc      *time.Location
		from, to string
	}{
		{"between is inclusive of the last day", "f=period:between:2026-11-10,2026-11-12", loc, "2026-11-10T00:00:00+05:00", "2026-11-13T00:00:00+05:00"},
		{"between without a zone is UTC", "f=period:between:2026-11-10,2026-11-12", nil, "2026-11-10T00:00:00Z", "2026-11-13T00:00:00Z"},
		{"on one day", "f=period:on:2026-11-10", loc, "2026-11-10T00:00:00+05:00", "2026-11-11T00:00:00+05:00"},
		{"before a day", "f=period:before:2026-11-10", loc, "", "2026-11-10T00:00:00+05:00"},
		{"after a day", "f=period:after:2026-11-10", loc, "2026-11-11T00:00:00+05:00", ""},
		{"after and before narrow each other", "f=period:after:2026-11-01&f=period:before:2026-11-10", loc, "2026-11-02T00:00:00+05:00", "2026-11-10T00:00:00+05:00"},
		{"this month resolves in the zone", "f=period:between:preset:this_month", loc, "2026-11-01T00:00:00+05:00", "2026-12-01T00:00:00+05:00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := parse(t, tt.query).Query(tt.loc, now)
			require.Equal(t, tt.from, q.From)
			require.Equal(t, tt.to, q.To)
		})
	}
}

func TestQuery_PresetUsesTheDayInTheApplicationZone(t *testing.T) {
	t.Parallel()
	lateOnThe30th := time.Date(2026, 11, 30, 22, 0, 0, 0, time.UTC)
	q := parse(t, "f=period:between:preset:this_month").Query(tashkent(t), lateOnThe30th)
	require.Equal(t, "2026-12-01T00:00:00+05:00", q.From, "22:00 UTC is already December in Tashkent")
	require.Equal(t, "2027-01-01T00:00:00+05:00", q.To)
}

func TestParseFilters_BareResourceBecomesACondition(t *testing.T) {
	t.Parallel()
	f := parse(t, "resource=finance.payment")
	require.Empty(t, f.ID)
	require.Empty(t, f.Resource)
	require.Equal(t, filterq.FilterSet{{Field: historypage.FieldResource, Op: filterq.OpIs, Values: []string{"finance.payment"}}}, f.Conditions)
	require.Equal(t, []string{"finance.payment"}, f.Query(nil, now).Resources)

	explicit := parse(t, "resource=finance.payment&fb=1&f=resource:is:finance.expense")
	require.Len(t, explicit.Conditions, 1, "an explicit condition wins over the link parameter")
	require.Equal(t, []string{"finance.expense"}, explicit.Query(nil, now).Resources)
}

func TestParseFilters_ObjectScopeIgnoresResourceCondition(t *testing.T) {
	t.Parallel()
	f := parse(t, "resource=finance.payment&id=104&f=resource:is:finance.expense&f=channel:is:mcp")
	q := f.Query(nil, now)
	require.Equal(t, loghistory.Scope{Kind: loghistory.ScopeEntity, Resource: "finance.payment", ID: "104"}, q.Scope)
	require.Nil(t, q.Resources)
	require.Equal(t, []string{"mcp"}, q.Channels)
}

func TestParseFilters_DropsInvalidConditions(t *testing.T) {
	t.Parallel()
	f := parse(t, "f=bogus:is:1&f=period:between:yesterday,today&f=channel:is:ui")
	require.Len(t, f.Conditions, 1)
	require.Equal(t, historypage.FieldChannel, f.Conditions[0].Field)
}

func TestFiltersURL_RoundTripsConditionsAndScope(t *testing.T) {
	t.Parallel()
	f := parse(t, "actor=7&actor_name=Anna&fb=1&f=resource:is:finance.expense,finance.payment&f=period:between:2026-11-01,2026-11-05&cursor=abc")

	link := f.URL("/admin/history")
	values := mustQuery(t, link)
	require.Empty(t, values.Get("cursor"), "a link starts the listing over")
	require.Equal(t, "7", values.Get("actor"))
	require.Equal(t, "1", values.Get("fb"))
	require.Equal(t, f.Conditions, historypage.ParseFilters(values, schema).Conditions)

	require.Equal(t, "/admin/history", f.Scope().URL("/admin/history"))
	cleared := mustQuery(t, f.URL("/admin/history", "actor", "", "actor_name", ""))
	require.Empty(t, cleared.Get("actor"))
	require.Len(t, cleared["f"], 2)

	pinned := parse(t, "resource=finance.payment&id=104&f=channel:is:ui")
	require.Equal(t, "/admin/history?id=104&resource=finance.payment", pinned.Scope().URL("/admin/history"))
}

func mustQuery(t *testing.T, link string) url.Values {
	t.Helper()
	parsed, err := url.Parse(link)
	require.NoError(t, err)
	return parsed.Query()
}
