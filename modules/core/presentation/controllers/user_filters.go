package controllers

import (
	"context"
	"net/url"
	"time"

	"github.com/iota-uz/iota-sdk/components/scaffold/filterbuilder"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/filterq"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

// Field keys for the Users list filterbuilder chip bar (see BuildTableConfig
// in templates/pages/users). Kept unexported: only user_filters.go and
// user_list_handlers.go need them.
const (
	userFilterFieldRole      = "roleID"
	userFilterFieldGroup     = "groupID"
	userFilterFieldCreatedAt = "createdAt"
)

// buildUserFilterRegistry builds the filterbuilder registry backing the Users
// list chip bar. Role and group are multi-select reference fields (OpIs
// accepts one or more values) sourced from the tenant's current roles/groups
// (with usage counts as chip badges).
// CreatedAt is a preset-only date range, matching the preset-dropdown UX of
// the sidebar filter it replaces — PresetNext30D is deliberately omitted
// (nonsensical for a past-only "created at" field) and operators are
// restricted to OpBetween only (no new "before X"/"on X"/"after X"
// capability is being introduced here).
func buildUserFilterRegistry(ctx context.Context, roles []*viewmodels.Role, groups []*viewmodels.Group) *filterbuilder.Registry {
	pageCtx := composables.UsePageCtx(ctx)

	roleOptions := make([]filterbuilder.Option, 0, len(roles))
	for _, role := range roles {
		roleOptions = append(roleOptions, filterbuilder.Option{
			Value: role.ID,
			Label: role.Name,
			Count: role.UsersCount,
		})
	}

	groupOptions := make([]filterbuilder.Option, 0, len(groups))
	for _, group := range groups {
		groupOptions = append(groupOptions, filterbuilder.Option{
			Value: group.ID,
			Label: group.Name,
			Count: group.UsersCount(),
		})
	}

	return filterbuilder.NewRegistry(
		filterbuilder.FieldDef{
			Key:       userFilterFieldRole,
			Type:      filterq.FieldTypeReference,
			Label:     pageCtx.T("Users.List.FilterRole"),
			Operators: []filterq.Operator{filterq.OpIs},
			Options:   roleOptions,
		},
		filterbuilder.FieldDef{
			Key:       userFilterFieldGroup,
			Type:      filterq.FieldTypeReference,
			Label:     pageCtx.T("Users.List.FilterGroup"),
			Operators: []filterq.Operator{filterq.OpIs},
			Options:   groupOptions,
		},
		filterbuilder.FieldDef{
			Key:       userFilterFieldCreatedAt,
			Type:      filterq.FieldTypeDate,
			Label:     pageCtx.T("Users.List.FilterCreatedAt"),
			Operators: []filterq.Operator{filterq.OpBetween},
			Presets: []filterq.DatePreset{
				filterq.PresetThisMonth,
				filterq.PresetLastMonth,
				filterq.PresetLast30D,
				filterq.PresetThisYear,
				filterq.PresetLastYear,
			},
		},
	)
}

// decodeUserFilterSet parses the request's `f`/`fb` query parameters against
// the registry's validation schema. Unknown fields/operators/malformed
// values are silently dropped by filterq.Decode.
func decodeUserFilterSet(reg *filterbuilder.Registry, q url.Values) filterq.FilterSet {
	return reg.Decode(q)
}

// applyUserFilterSet translates a decoded FilterSet into query.Filter entries
// on findParams, using the exact same query.Filter{Column, Filter} shape
// user_query_repository.go already consumes (buildRoleFilterCondition /
// buildGroupFilterCondition / the FieldCreatedAt column filter) — that file
// is not modified.
func applyUserFilterSet(now time.Time, fs filterq.FilterSet, findParams *query.FindParams) {
	appendInFilter(findParams, fs, userFilterFieldRole, query.FieldRoleID)
	appendInFilter(findParams, fs, userFilterFieldGroup, query.FieldGroupID)

	for _, cond := range fs.Field(userFilterFieldCreatedAt) {
		from, to, ok := cond.DateRange(now)
		if !ok {
			continue
		}
		// DateRange resolves to inclusive date-only bounds (both at
		// midnight); extend the upper bound to the next day's midnight so
		// the "to" day is included in full, matching the inclusive-day
		// semantics of the sidebar/dropdown filter this replaces.
		findParams.Filters = append(findParams.Filters,
			query.Filter{Column: query.FieldCreatedAt, Filter: repo.Gte(from)},
			query.Filter{Column: query.FieldCreatedAt, Filter: repo.Lt(to.AddDate(0, 0, 1))},
		)
	}
}

// appendInFilter applies an OpIs condition (one or more selected values) from
// fieldKey as an "IN" filter on column, shared by the role and group blocks
// in applyUserFilterSet since both follow the exact same shape.
func appendInFilter(findParams *query.FindParams, fs filterq.FilterSet, fieldKey string, column query.Field) {
	for _, cond := range fs.Field(fieldKey) {
		if cond.Op != filterq.OpIs {
			continue
		}
		findParams.Filters = append(findParams.Filters, query.Filter{
			Column: column,
			Filter: repo.In(cond.Values),
		})
	}
}
