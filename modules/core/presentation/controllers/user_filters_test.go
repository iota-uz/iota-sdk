package controllers

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/filterq"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// stubPageCtx satisfies types.PageContext returning the key itself for every
// translation call, mirroring components/scaffold/filterbuilder's test stub.
// It lets buildUserFilterRegistry run outside a full HTTP/itf harness.
type stubPageCtx struct{}

func (stubPageCtx) T(key string, _ ...map[string]interface{}) string     { return key }
func (stubPageCtx) TSafe(key string, _ ...map[string]interface{}) string { return key }
func (s stubPageCtx) Namespace(string) types.PageContext                 { return s }
func (stubPageCtx) ToJSLocale() string                                   { return "en-US" }
func (stubPageCtx) GetLocale() language.Tag                              { return language.English }
func (stubPageCtx) GetURL() *url.URL                                     { return &url.URL{Path: "/users"} }
func (stubPageCtx) GetLocalizer() *i18n.Localizer                        { return nil }

func filterTestCtx() context.Context {
	return composables.WithPageCtx(context.Background(), stubPageCtx{})
}

func TestBuildUserFilterRegistry(t *testing.T) {
	t.Parallel()

	roles := []*viewmodels.Role{
		{ID: "1", Name: "Admin", UsersCount: 3},
		{ID: "2", Name: "Manager", UsersCount: 0},
	}
	groups := []*viewmodels.Group{
		{ID: "group-1", Name: "Sales"},
	}

	reg := buildUserFilterRegistry(filterTestCtx(), roles, groups)
	require.NotNil(t, reg)

	roleField, ok := reg.Field(userFilterFieldRole)
	require.True(t, ok)
	assert.Equal(t, filterq.FieldTypeReference, roleField.Type)
	assert.Equal(t, []filterq.Operator{filterq.OpIs}, roleField.Operators)
	require.Len(t, roleField.Options, 2)
	assert.Equal(t, "1", roleField.Options[0].Value)
	assert.Equal(t, "Admin", roleField.Options[0].Label)
	assert.Equal(t, 3, roleField.Options[0].Count)

	groupField, ok := reg.Field(userFilterFieldGroup)
	require.True(t, ok)
	assert.Equal(t, filterq.FieldTypeReference, groupField.Type)
	require.Len(t, groupField.Options, 1)
	assert.Equal(t, "group-1", groupField.Options[0].Value)
	assert.Equal(t, "Sales", groupField.Options[0].Label)

	createdAtField, ok := reg.Field(userFilterFieldCreatedAt)
	require.True(t, ok)
	assert.Equal(t, filterq.FieldTypeDate, createdAtField.Type)
	assert.Equal(t, []filterq.Operator{filterq.OpBetween}, createdAtField.Operators)
	assert.ElementsMatch(t, []filterq.DatePreset{
		filterq.PresetThisMonth,
		filterq.PresetLastMonth,
		filterq.PresetLast30D,
		filterq.PresetThisYear,
		filterq.PresetLastYear,
	}, createdAtField.Presets)
	assert.NotContains(t, createdAtField.Presets, filterq.PresetNext30D)
}

func TestDecodeUserFilterSet(t *testing.T) {
	t.Parallel()

	reg := buildUserFilterRegistry(filterTestCtx(),
		[]*viewmodels.Role{{ID: "1", Name: "Admin"}},
		[]*viewmodels.Group{{ID: "group-1", Name: "Sales"}},
	)

	q := url.Values{}
	q.Add(filterq.ParamName, "roleID:is:1")
	q.Add(filterq.ParamName, "groupID:is:group-1")
	q.Add(filterq.ParamName, "createdAt:between:preset:this_month")
	q.Set(filterq.PresenceParam, "1")

	fs := decodeUserFilterSet(reg, q)

	require.Len(t, fs.Field(userFilterFieldRole), 1)
	assert.Equal(t, []string{"1"}, fs.Field(userFilterFieldRole)[0].Values)

	require.Len(t, fs.Field(userFilterFieldGroup), 1)
	assert.Equal(t, []string{"group-1"}, fs.Field(userFilterFieldGroup)[0].Values)

	require.Len(t, fs.Field(userFilterFieldCreatedAt), 1)
	preset, ok := fs.Field(userFilterFieldCreatedAt)[0].Preset()
	require.True(t, ok)
	assert.Equal(t, filterq.PresetThisMonth, preset)
}

// TestDecodeUserFilterSet_UnknownFieldDropped verifies filterq.Decode's
// schema validation drops conditions for fields not present in the registry
// (e.g. a stale/tampered `f` param).
func TestDecodeUserFilterSet_UnknownFieldDropped(t *testing.T) {
	t.Parallel()

	reg := buildUserFilterRegistry(filterTestCtx(), nil, nil)

	q := url.Values{}
	q.Add(filterq.ParamName, "notAField:is:1")

	fs := decodeUserFilterSet(reg, q)
	assert.True(t, fs.IsZero())
}

func TestApplyUserFilterSet_RoleAndGroup(t *testing.T) {
	t.Parallel()

	fs := filterq.FilterSet{
		{Field: userFilterFieldRole, Op: filterq.OpIs, Values: []string{"1", "2"}},
		{Field: userFilterFieldGroup, Op: filterq.OpIs, Values: []string{"group-1"}},
	}

	findParams := &query.FindParams{Filters: []query.Filter{}}
	applyUserFilterSet(time.Now(), fs, findParams)

	require.Len(t, findParams.Filters, 2)

	roleFilter := findParams.Filters[0]
	assert.Equal(t, query.FieldRoleID, roleFilter.Column)
	assert.Equal(t, []any{"1", "2"}, roleFilter.Filter.Value())

	groupFilter := findParams.Filters[1]
	assert.Equal(t, query.FieldGroupID, groupFilter.Column)
	assert.Equal(t, []any{"group-1"}, groupFilter.Filter.Value())
}

func TestApplyUserFilterSet_CreatedAt_ExplicitRange(t *testing.T) {
	t.Parallel()

	fs := filterq.FilterSet{
		{Field: userFilterFieldCreatedAt, Op: filterq.OpBetween, Values: []string{"2024-01-10", "2024-01-12"}},
	}

	findParams := &query.FindParams{Filters: []query.Filter{}}
	applyUserFilterSet(time.Now(), fs, findParams)

	require.Len(t, findParams.Filters, 2)

	gteFilter := findParams.Filters[0]
	assert.Equal(t, query.FieldCreatedAt, gteFilter.Column)
	from := gteFilter.Filter.Value()[0].(time.Time)
	assert.Equal(t, "2024-01-10", from.Format("2006-01-02"))

	ltFilter := findParams.Filters[1]
	assert.Equal(t, query.FieldCreatedAt, ltFilter.Column)
	to := ltFilter.Filter.Value()[0].(time.Time)
	// Upper bound is exclusive and extended by one day so the "to" day
	// (2024-01-12) is included in full.
	assert.Equal(t, "2024-01-13", to.Format("2006-01-02"))
}

func TestApplyUserFilterSet_CreatedAt_Preset(t *testing.T) {
	t.Parallel()

	now := time.Date(2024, time.March, 15, 12, 0, 0, 0, time.UTC)
	fs := filterq.FilterSet{
		{Field: userFilterFieldCreatedAt, Op: filterq.OpBetween, Values: []string{"preset:this_month"}},
	}

	findParams := &query.FindParams{Filters: []query.Filter{}}
	applyUserFilterSet(now, fs, findParams)

	require.Len(t, findParams.Filters, 2)

	from := findParams.Filters[0].Filter.Value()[0].(time.Time)
	assert.Equal(t, "2024-03-01", from.Format("2006-01-02"))

	to := findParams.Filters[1].Filter.Value()[0].(time.Time)
	// This month is March 2024 (1st..31st); the upper bound is midnight of
	// the day after the last day of the month.
	assert.Equal(t, "2024-04-01", to.Format("2006-01-02"))
}

func TestApplyUserFilterSet_Empty(t *testing.T) {
	t.Parallel()

	findParams := &query.FindParams{Filters: []query.Filter{}}
	applyUserFilterSet(time.Now(), filterq.FilterSet{}, findParams)

	assert.Empty(t, findParams.Filters)
}
