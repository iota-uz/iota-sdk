package controllers

import (
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/users"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

func parseFilterTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

func userIndexFiltersFromRequest(r *http.Request) users.IndexFilterState {
	return users.IndexFilterState{
		Search:        r.URL.Query().Get("Search"),
		RoleIDs:       r.URL.Query()["roleID"],
		GroupIDs:      r.URL.Query()["groupID"],
		CreatedAtFrom: r.URL.Query().Get("CreatedAt.From"),
		CreatedAtTo:   r.URL.Query().Get("CreatedAt.To"),
	}
}

func (c *UsersController) Users(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userQueryService *services.UserQueryService,
	groupQueryService *services.GroupQueryService,
	roleQueryService *services.RoleQueryService,
) {
	params := composables.UsePaginated(r)
	filters := userIndexFiltersFromRequest(r)

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
		Search:  filters.Search,
		Filters: []query.Filter{},
	}

	if len(filters.GroupIDs) > 0 {
		findParams.Filters = append(findParams.Filters, query.Filter{
			Column: query.FieldGroupID,
			Filter: repo.In(filters.GroupIDs),
		})
	}

	if len(filters.RoleIDs) > 0 {
		findParams.Filters = append(findParams.Filters, query.Filter{
			Column: query.FieldRoleID,
			Filter: repo.In(filters.RoleIDs),
		})
	}

	if filters.CreatedAtTo != "" {
		t, err := parseFilterTime(filters.CreatedAtTo)
		if err != nil {
			logger.WithError(err).Error("error parsing CreatedAt.To")
			http.Error(w, "Invalid date format", http.StatusBadRequest)
			return
		}
		findParams.Filters = append(findParams.Filters, query.Filter{
			Column: query.FieldCreatedAt,
			Filter: repo.Lt(t),
		})
	}

	if filters.CreatedAtFrom != "" {
		t, err := parseFilterTime(filters.CreatedAtFrom)
		if err != nil {
			logger.WithError(err).Error("error parsing CreatedAt.From")
			http.Error(w, "Invalid date format", http.StatusBadRequest)
			return
		}
		findParams.Filters = append(findParams.Filters, query.Filter{
			Column: query.FieldCreatedAt,
			Filter: repo.Gte(t),
		})
	}

	userViewModels, total, err := userQueryService.FindUsers(r.Context(), findParams)
	if err != nil {
		logger.WithError(err).Error("error retrieving users")
		http.Error(w, "Error retrieving users", http.StatusInternalServerError)
		return
	}

	groups, _, err := groupQueryService.FindGroups(r.Context(), &query.GroupFindParams{
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

	roleViewModels, err := roleQueryService.GetRolesWithCounts(r.Context())
	if err != nil {
		logger.WithError(err).Error("error retrieving roles")
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}

	props := &users.IndexPageProps{
		Users:   userViewModels,
		Groups:  groups,
		Roles:   roleViewModels,
		Page:    params.Page,
		PerPage: params.Limit,
		HasMore: total > params.Page*params.Limit,
		Filters: filters,
	}

	if htmx.IsHxRequest(r) {
		if params.Page > 1 || htmx.Target(r) == "users-table-body" {
			templ.Handler(users.UserRows(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}

		templ.Handler(users.UsersPageContent(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	templ.Handler(users.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
}
