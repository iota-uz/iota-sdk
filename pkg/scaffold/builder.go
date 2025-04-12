package scaffold

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-faster/errors"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/components/scaffold"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// TableService defines the minimal interface for table data services
type TableService[T any] interface {
	GetPaginated(ctx context.Context, params interface{}) ([]T, error)
	Count(ctx context.Context, params interface{}) (int64, error)
}

// TableViewModel defines the interface for mapping entity to view model
type TableViewModel[T any] interface {
	MapToViewModel(entity T) map[string]interface{}
}

// TableControllerBuilder helps to quickly build controllers for displaying tables
type TableControllerBuilder[T any] struct {
	app          application.Application
	service      TableService[T]
	viewModel    TableViewModel[T]
	basePath     string
	tableConfig  *scaffold.TableConfig
	findParamsFn func(r *http.Request) interface{}
}

// NewTableControllerBuilder creates a new table controller builder
func NewTableControllerBuilder[T any](
	app application.Application,
	service TableService[T],
	viewModel TableViewModel[T],
	basePath string,
	tableConfig *scaffold.TableConfig,
) *TableControllerBuilder[T] {
	return &TableControllerBuilder[T]{
		app:         app,
		service:     service,
		viewModel:   viewModel,
		basePath:    basePath,
		tableConfig: tableConfig,
		findParamsFn: func(r *http.Request) interface{} {
			params := composables.UsePaginated(r)
			return &struct {
				Limit  int
				Offset int
				Search string
				SortBy []string
			}{
				Limit:  params.Limit,
				Offset: params.Offset,
				Search: r.URL.Query().Get("search"),
				SortBy: []string{"id"},
			}
		},
	}
}

func (b *TableControllerBuilder[T]) Key() string {
	return b.basePath
}

// WithFindParamsFunc sets a custom function for creating find parameters
func (b *TableControllerBuilder[T]) WithFindParamsFunc(fn func(r *http.Request) interface{}) *TableControllerBuilder[T] {
	b.findParamsFn = fn
	return b
}

// Register registers the table route
func (b *TableControllerBuilder[T]) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(b.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}

	// Register the GET route for the table
	getRouter := r.PathPrefix(b.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", b.List).Methods(http.MethodGet)
}

// List handles listing entities in a table
func (b *TableControllerBuilder[T]) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pageCtx := composables.UsePageCtx(ctx)

	// Get parameters for the query
	params := b.findParamsFn(r)

	// Extract search from params
	var search string
	if paramsMap, ok := params.(map[string]interface{}); ok && paramsMap["search"] != nil {
		search = paramsMap["search"].(string)
	} else if paramsStruct, ok := params.(struct{ Search string }); ok {
		search = paramsStruct.Search
	} else {
		// Default to query parameter
		search = r.URL.Query().Get("search")
	}

	// Get entities from the service
	entities, err := b.service.GetPaginated(ctx, params)
	if err != nil {
		http.Error(w, errors.Wrap(err, "Error retrieving entities").Error(), http.StatusInternalServerError)
		return
	}

	// Count for pagination
	total, err := b.service.Count(ctx, search)
	if err != nil {
		http.Error(w, errors.Wrap(err, "Error counting entities").Error(), http.StatusInternalServerError)
		return
	}

	// Get pagination parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	// Default limit is 10
	var limit int64 = 10
	if paramsMap, ok := params.(map[string]interface{}); ok && paramsMap["limit"] != nil {
		limit = paramsMap["limit"].(int64)
	}

	// Calculate total pages
	totalPages := (total + limit - 1) / limit

	// Map entities to view models and create table data
	data := scaffold.NewData()
	for _, entity := range entities {
		viewModel := b.viewModel.MapToViewModel(entity)
		data.AddItem(viewModel)
	}

	// Check if this is an HTMX request
	isHxRequest := htmx.IsHxRequest(r)

	if isHxRequest {
		// Render just the table component
		_ = ExtendedTable(
			b.tableConfig,
			data,
			page,
			int(totalPages),
			pageCtx,
		).Render(ctx, w)
	} else {
		// Render the full page
		content := ExtendedContent(
			b.tableConfig,
			data,
			search,
			page,
			int(totalPages),
			pageCtx,
		)
		_ = PageWithLayout(content, pageCtx).Render(ctx, w)
	}
}

func NewSimpleTableViewModel[T any](mapFn func(a T) map[string]interface{}) TableViewModel[T] {
	return &simpleTableViewModel[T]{
		mapFn: mapFn,
	}
}

type simpleTableViewModel[T any] struct {
	mapFn func(a T) map[string]interface{}
}

func (v *simpleTableViewModel[T]) MapToViewModel(entity T) map[string]interface{} {
	return v.mapFn(entity)
}
