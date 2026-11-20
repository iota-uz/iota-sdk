package filterq

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sqlSchema = Schema{Fields: []Field{
	{Key: "account", Type: FieldTypeReference, Operators: NullableOperators(FieldTypeReference)},
	{Key: "date", Type: FieldTypeDate},
	{Key: "amount", Type: FieldTypeNumber},
	{Key: "active", Type: FieldTypeBool},
	{Key: "overdue", Type: FieldTypeBool},
	{Key: "skipped", Type: FieldTypeReference},
}}

var sqlColumns = map[string]Column{
	"account": {Expr: "t.account_id", Type: "uuid"},
	"date":    {Expr: "t.created_at"},
	"amount":  {Expr: "abs(t.amount)"},
	"active":  {Expr: "a.is_active"},
	"overdue": {Custom: func(c Condition, p *Params) (string, bool) {
		if c.MatchBool(true) {
			return "d.due < " + p.Add("2026-01-01") + "::date", true
		}
		return "", false
	}},
}

func TestAppendSQL(t *testing.T) {
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		f    []string
		sql  string
		args []any
	}{
		{"none", nil, "", []any{"tenant"}},
		{"is any of", []string{"account:is:a,b"}, " AND (t.account_id = ANY($2::uuid[]))", []any{"tenant", []string{"a", "b"}}},
		{"is not keeps blanks", []string{"account:isnot:a"}, " AND (NOT COALESCE(t.account_id = ANY($2::uuid[]), false))", []any{"tenant", []string{"a"}}},
		{"empty", []string{"account:empty:"}, " AND (t.account_id IS NULL)", []any{"tenant"}},
		{"date preset", []string{"date:between:preset:this_month"}, " AND ((t.created_at)::date BETWEEN $2::date AND $3::date)", []any{"tenant", "2026-03-01", "2026-03-31"}},
		{"date before", []string{"date:before:2026-02-01"}, " AND ((t.created_at)::date < $2::date)", []any{"tenant", "2026-02-01"}},
		{"amount range", []string{"amount:between:100,500"}, " AND ((abs(t.amount))::numeric BETWEEN SYMMETRIC $2::numeric AND $3::numeric)", []any{"tenant", "100", "500"}},
		{"bool false", []string{"active:is:false"}, " AND (a.is_active IS NOT TRUE)", []any{"tenant"}},
		{"custom", []string{"overdue:is:true"}, " AND (d.due < $2::date)", []any{"tenant", "2026-01-01"}},
		{"custom declines", []string{"overdue:is:false"}, "", []any{"tenant"}},
		{"field without column", []string{"skipped:is:x"}, "", []any{"tenant"}},
		{"conditions are and-ed", []string{"account:is:a", "amount:gt:10"},
			" AND (t.account_id = ANY($2::uuid[])) AND ((abs(t.amount))::numeric > $3::numeric)",
			[]any{"tenant", []string{"a"}, "10"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := Decode(url.Values{ParamName: tt.f}, sqlSchema)
			require.Len(t, fs, len(tt.f))
			sql, args := AppendSQL(fs, sqlSchema, sqlColumns, now, []any{"tenant"})
			assert.Equal(t, tt.sql, sql)
			assert.Equal(t, tt.args, args)
		})
	}
}

func TestEmptyOperatorCodec(t *testing.T) {
	c := Condition{Field: "account", Op: OpEmpty}
	assert.Equal(t, "account:empty:", EncodeCondition(c))
	fs := Decode(url.Values{ParamName: {"account:empty:", "date:empty:"}}, sqlSchema)
	// date does not opt into OpEmpty, so only account survives.
	assert.Equal(t, FilterSet{c}, fs)
	assert.Equal(t, 0, OpEmpty.Arity())
}

func TestMatch(t *testing.T) {
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	is := Condition{Op: OpIs, Values: []string{"a", "b"}}
	isNot := Condition{Op: OpIsNot, Values: []string{"a"}}
	empty := Condition{Op: OpEmpty}
	thisMonth := Condition{Op: OpBetween, Values: []string{"preset:this_month"}}
	before := Condition{Op: OpBefore, Values: []string{"2026-03-10"}}
	rng := Condition{Op: OpBetween, Values: []string{"500", "100"}}
	tests := []struct {
		name string
		got  bool
		want bool
	}{
		{"is listed", is.MatchValue("b"), true},
		{"is unlisted", is.MatchValue("c"), false},
		{"is blank", is.MatchValue(""), false},
		{"isnot blank", isNot.MatchValue(""), true},
		{"isnot listed", isNot.MatchValue("a"), false},
		{"empty blank", empty.MatchValue(""), true},
		{"empty set", empty.MatchValue("a"), false},
		{"values any", is.MatchValues([]string{"x", "a"}), true},
		{"values none", isNot.MatchValues([]string{"x", "a"}), false},
		{"date in preset", thisMonth.MatchDate(time.Date(2026, 3, 31, 23, 0, 0, 0, time.UTC), now), true},
		{"date out of preset", thisMonth.MatchDate(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), now), false},
		{"date before", before.MatchDate(time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC), now), true},
		{"date before same day", before.MatchDate(time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), now), false},
		{"zero date empty", empty.MatchDate(time.Time{}, now), true},
		{"zero date range", thisMonth.MatchDate(time.Time{}, now), false},
		{"number reversed range", rng.MatchNumber(300), true},
		{"number edge", rng.MatchNumber(500), true},
		{"number outside", rng.MatchNumber(501), false},
		{"bool", Condition{Op: OpIs, Values: []string{"false"}}.MatchBool(false), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { assert.Equal(t, tt.want, tt.got) })
	}
}

func TestRepoFiltersRenderAgainstRepositoryColumn(t *testing.T) {
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	fs := Decode(url.Values{ParamName: {"account:is:a,b", "amount:between:1,2", "skipped:is:x"}}, sqlSchema)
	filters := RepoFilters(fs, sqlSchema, map[string]RepoField[int]{
		"account": {Column: 7, Type: "uuid"},
		"amount":  {Column: 9},
	}, now)
	require.Len(t, filters, 2)
	assert.Equal(t, 7, filters[0].Column)
	assert.Equal(t, "(ma.account_id = ANY($4::uuid[]))", filters[0].Filter.String("ma.account_id", 4))
	assert.Equal(t, []any{[]string{"a", "b"}}, filters[0].Filter.Value())
	assert.Equal(t, "((ma.balance)::numeric BETWEEN SYMMETRIC $2::numeric AND $3::numeric)", filters[1].Filter.String("ma.balance", 2))
	assert.Equal(t, []any{"1", "2"}, filters[1].Filter.Value())
}

func TestLegacyDateRange(t *testing.T) {
	q := url.Values{"CreatedAt.From": {"2026-01-01T00:00:00Z"}, "CreatedAt.To": {"2026-01-31"}}
	out := LegacyDateRange(q, "CreatedAt.From", "CreatedAt.To", "created_at")
	assert.Equal(t, []string{"created_at:between:2026-01-01,2026-01-31"}, out[ParamName])
	assert.Empty(t, q[ParamName], "the input is not modified")
	assert.Equal(t, q, LegacyDateRange(q, "CreatedAt.From", "Missing", "created_at"))
}
