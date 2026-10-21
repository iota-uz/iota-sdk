package filtercontext_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/filtercontext"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stretchr/testify/require"
)

func baseline() filtercontext.ContextInput {
	return filtercontext.ContextInput{Calendar: filtercontext.ContextCalendar{Timezone: "Asia/Tashkent", AsOf: "2026-05-31T20:30:00Z", DefaultPeriod: filtercontext.ContextPeriod{Mode: "relative", Days: 30}}, SupportedDimensions: []string{"product"}}
}

// False green: merge dimensions by last-write-wins, or resolve relative dates in UTC.
func TestScopePriorityIntersectionAndCalendarSurviveRoundTrip(t *testing.T) {
	t.Parallel()
	input := baseline()
	input.Layers = []filtercontext.ContextLayer{
		{Origin: filtercontext.ContextOrigin{Scope: "card", ID: "first"}, Filters: filtercontext.ContextFilters{Period: &filtercontext.ContextPeriod{Mode: "fixed", Start: "2026-06-01", End: "2026-06-30"}, Constraints: []filtercontext.ContextConstraint{{Dimension: "product", Values: []string{"B", "A"}}}}},
		{Origin: filtercontext.ContextOrigin{Scope: "report", ID: "sales"}, Filters: filtercontext.ContextFilters{Period: &filtercontext.ContextPeriod{Mode: "relative", Days: 30}, Constraints: []filtercontext.ContextConstraint{{Dimension: "product", Values: []string{"A", "C"}}}}},
		{Origin: filtercontext.ContextOrigin{Scope: "dashboard", ID: "annual"}, Filters: filtercontext.ContextFilters{Period: &filtercontext.ContextPeriod{Mode: "fixed", Start: "2026-01-01", End: "2026-12-31"}}},
	}
	opened, err := filtercontext.Resolve(input)
	require.NoError(t, err)
	require.Equal(t, "2026-06-01", opened.Period.Start)
	require.Equal(t, "card", opened.Period.Origin.Scope)
	require.Equal(t, []string{"A"}, opened.Intersection[0].Values)
	require.Len(t, opened.Restrictions, 2)
	encoded, err := filtercontext.Encode(input)
	require.NoError(t, err)
	decoded, err := filtercontext.Decode(encoded)
	require.NoError(t, err)
	roundTrip, err := filtercontext.Resolve(decoded)
	require.NoError(t, err)
	require.Equal(t, opened.Identity, roundTrip.Identity)
	input.Layers[0].Filters.Constraints[0].Values[0] = "changed"
	require.Equal(t, []string{"A"}, opened.Intersection[0].Values)
	defaulted, err := filtercontext.Resolve(baseline())
	require.NoError(t, err)
	require.Equal(t, "2026-06-01", defaulted.Period.End)
	require.Equal(t, "2026-05-03", defaulted.Period.Start)
	require.Equal(t, "relative", defaulted.Period.Selection.Mode)
}

// False green: delete an incompatible predicate or pop a dashboard/report default on Back.
func TestConflictUnsupportedAndBackRetainOrigins(t *testing.T) {
	t.Parallel()
	input := baseline()
	input.Layers = []filtercontext.ContextLayer{{Origin: filtercontext.ContextOrigin{Scope: "report", ID: "r"}, Filters: filtercontext.ContextFilters{Constraints: []filtercontext.ContextConstraint{{Dimension: "product", Values: []string{"A"}}}}}}
	group, err := filtercontext.PushGroup(input, "june", filtercontext.ContextFilters{Constraints: []filtercontext.ContextConstraint{{Dimension: "product", Values: []string{"B"}}, {Dimension: "seller", Values: []string{"42"}}}})
	require.NoError(t, err)
	require.False(t, group.Ready())
	require.Equal(t, []string{"product"}, group.Conflicts)
	require.Len(t, group.Restrictions, 3)
	require.Equal(t, "seller", group.Unsupported[0].Dimension)
	back, err := filtercontext.Back(group.Input)
	require.NoError(t, err)
	require.True(t, back.Ready())
	require.Equal(t, []string{"A"}, back.Intersection[0].Values)
	require.Equal(t, "report", back.Restrictions[0].Origin.Scope)
}

// False green: confuse report-default inheritance with an all-time override.
func TestReportsModeIsNotAllTime(t *testing.T) {
	t.Parallel()
	input := baseline()
	input.Layers = []filtercontext.ContextLayer{{Origin: filtercontext.ContextOrigin{Scope: "dashboard", ID: "d"}, Filters: filtercontext.ContextFilters{Period: &filtercontext.ContextPeriod{Mode: "reports"}}}}
	inherited, err := filtercontext.Resolve(input)
	require.NoError(t, err)
	require.NotEmpty(t, inherited.Period.Start)
	input.Layers[0].Filters.Period = &filtercontext.ContextPeriod{Mode: "all"}
	all, err := filtercontext.Resolve(input)
	require.NoError(t, err)
	require.Empty(t, all.Period.Start)
	require.NotEqual(t, inherited.Identity, all.Identity)
	input.Calendar.Timezone = ""
	_, err = filtercontext.Resolve(input)
	require.True(t, serrors.HasCode(err, serrors.Invalid))
}
