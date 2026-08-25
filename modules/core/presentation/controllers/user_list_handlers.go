package controllers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/components/scaffold/table"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/users"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

func (c *UsersController) Users(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userQueryService *services.UserQueryService,
	groupQueryService *services.GroupQueryService,
	roleQueryService *services.RoleQueryService,
	userService *services.UserService,
	policy *services.PrivilegeGrantPolicy,
) {
	ctx := r.Context()
	params := composables.UsePaginated(r)
	search := r.URL.Query().Get("Search")

	// Roles/groups are fetched first: buildUserFilterRegistry needs their
	// data as filterbuilder chip Options (with usage counts).
	groups, _, err := groupQueryService.FindGroups(ctx, &query.GroupFindParams{
		Limit:  100,
		Offset: 0,
		SortBy: query.SortBy{
			Fields: []repo.SortByField[query.Field]{
				{Field: query.GroupFieldName, Ascending: true},
			},
		},
		Filters: []query.GroupFilter{},
	})
	if err != nil {
		logger.WithError(err).Error("error retrieving groups")
		http.Error(w, "Error retrieving groups", http.StatusInternalServerError)
		return
	}

	roleViewModels, err := roleQueryService.GetRolesWithCounts(ctx)
	if err != nil {
		logger.WithError(err).Error("error retrieving roles")
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}

	registry := buildUserFilterRegistry(ctx, roleViewModels, groups)
	filterSet := decodeUserFilterSet(registry, r.URL.Query())

	findParams := &query.FindParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		SortBy: query.SortBy{
			Fields: []repo.SortByField[query.Field]{
				{
					Field:     query.FieldCreatedAt,
					Ascending: false,
				},
			},
		},
		Search:  search,
		Filters: []query.Filter{},
	}
	applyUserFilterSet(time.Now(), filterSet, findParams)

	userViewModels, total, err := userQueryService.FindUsers(ctx, findParams)
	if err != nil {
		logger.WithError(err).Error("error retrieving users")
		http.Error(w, "Error retrieving users", http.StatusInternalServerError)
		return
	}

	actor, err := composables.UseUser(ctx)
	if err != nil {
		logger.WithError(err).Error("error retrieving current user")
		http.Error(w, "Error retrieving current user", http.StatusInternalServerError)
		return
	}
	for _, userViewModel := range userViewModels {
		userID, parseErr := strconv.ParseUint(userViewModel.ID, 10, 64)
		if parseErr != nil {
			userViewModel.CanUpdate = false
			userViewModel.CanDelete = false
			continue
		}
		target, loadErr := userService.GetByID(ctx, uint(userID))
		if loadErr != nil {
			logger.WithField("userID", userViewModel.ID).WithError(loadErr).Warn("failed to evaluate user management actions")
			userViewModel.CanUpdate = false
			userViewModel.CanDelete = false
			continue
		}
		canManage := policy.CanManageUser(actor, target)
		userViewModel.CanUpdate = userViewModel.CanUpdate && canManage
		userViewModel.CanDelete = userViewModel.CanDelete && canManage
	}

	props := &users.IndexPageProps{
		Users:    userViewModels,
		Groups:   groups,
		Roles:    roleViewModels,
		Page:     params.Page,
		PerPage:  params.Limit,
		HasMore:  total > params.Page*params.Limit,
		Search:   search,
		Registry: registry,
		Filters:  filterSet,
	}

	cfg := users.BuildTableConfig(ctx, props, r)
	templ.Handler(table.ContentHTMX(cfg), templ.WithStreaming()).ServeHTTP(w, r)
}
