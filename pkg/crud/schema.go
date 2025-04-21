package crud

import (
	"context"
	"encoding"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	formui "github.com/iota-uz/iota-sdk/components/scaffold/form"
	sfui "github.com/iota-uz/iota-sdk/components/scaffold/table"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

// parseID converts the incoming URL string into the generic ID type.
func parseID[ID any](idStr string) (ID, error) {
	var id ID
	// first, handle the common scalar cases via a type switch on the zero-value
	switch any(id).(type) {
	case string:
		return any(idStr).(ID), nil
	case int:
		v, err := strconv.Atoi(idStr)
		return any(v).(ID), err
	case int64:
		v, err := strconv.ParseInt(idStr, 10, 64)
		return any(v).(ID), err
	case uint, uint64:
		v, err := strconv.ParseUint(idStr, 10, 64)
		// strconv.ParseUint returns a uint64; convert down if needed
		if err != nil {
			return id, err
		}
		// perform safe narrowing if ID is uint
		switch any(id).(type) {
		case uint:
			return any(uint(v)).(ID), nil
		default:
			return any(v).(ID), nil
		}
	}

	// next, see if *ID implements TextUnmarshaler (e.g. for UUID types)
	ptr := reflect.New(reflect.TypeOf(id))
	if tu, ok := ptr.Interface().(encoding.TextUnmarshaler); ok {
		if err := tu.UnmarshalText([]byte(idStr)); err != nil {
			return id, fmt.Errorf("invalid %T: %w", id, err)
		}
		// ptr is *ID; we need the value
		return ptr.Elem().Interface().(ID), nil
	}

	return id, fmt.Errorf("unsupported ID type %T", id)
}

// SortBy and Filter are generic aliases using Field
type SortBy = repo.SortBy[string]
type Filter = repo.FieldFilter[string]

// FindParams defines pagination, sorting, searching, and filtering parameters
type FindParams struct {
	Limit   int
	Offset  int
	Search  string
	SortBy  SortBy
	Filters []Filter
}

// DataStore abstracts CRUD operations for type T with ID type ID
type DataStore[T any, ID any] interface {
	List(ctx context.Context, params FindParams) ([]T, error)
	Get(ctx context.Context, id ID) (T, error)
	Create(ctx context.Context, entity T) (ID, error)
	Update(ctx context.Context, id ID, entity T) error
	Delete(ctx context.Context, id ID) error
}

// ModelLevelValidator for full-model checks
type ModelLevelValidator[T any] interface {
	ValidateModel(ctx context.Context, model T) error
}

// Schema defines a runtime-driven CRUD resource for entity type T with identifier type ID.
type Schema[T any, ID any] struct {
	Name            string
	Path            string
	IDField         string
	Fields          []formui.Field
	Store           DataStore[T, ID]
	modelValidators []ModelLevelValidator[T]
	middlewares     []mux.MiddlewareFunc
}

// Ensure Schema implements application.Controller
var _ application.Controller = (*Schema[any, any])(nil)

// SchemaOpt configures optional settings on a Schema
type SchemaOpt[T any, ID any] func(s *Schema[T, ID])

// NewSchema constructs a new CRUD Schema and applies options
func NewSchema[T any, ID any](
	name, path, idField string,
	store DataStore[T, ID],
	opts ...SchemaOpt[T, ID],
) *Schema[T, ID] {
	s := &Schema[T, ID]{
		Name:    name,
		Path:    path,
		IDField: idField,
		Store:   store,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// WithFields sets the form fields for the Schema
func WithFields[T any, ID any](fields ...formui.Field) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.Fields = fields
	}
}

// WithModelValidators adds model-level validators
func WithModelValidators[T any, ID any](vs ...ModelLevelValidator[T]) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.modelValidators = vs
	}
}

// WithMiddlewares adds middleware functions to the Schema
func WithMiddlewares[T any, ID any](ms ...mux.MiddlewareFunc) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.middlewares = append(s.middlewares, ms...)
	}
}

// Register mounts CRUD HTTP handlers on the provided router
func (s *Schema[T, ID]) Register(r *mux.Router) {
	subR := r.PathPrefix(s.Path).Subrouter()
	subR.Use(s.middlewares...)
	subR.HandleFunc("", s.listHandler).Methods(http.MethodGet)
	subR.HandleFunc("/new", s.newHandler).Methods(http.MethodGet)
	subR.HandleFunc("/", s.createHandler).Methods(http.MethodPost)
	subR.HandleFunc("/{id}/edit", s.editHandler).Methods(http.MethodGet)
	subR.HandleFunc("/{id}", s.updateHandler).Methods(http.MethodPut)
	subR.HandleFunc("/{id}", s.deleteHandler).Methods(http.MethodDelete)
}

// Key returns the base path for routing identification
func (s *Schema[T, ID]) Key() string {
	return s.Path
}

// Handler stubs
func (s *Schema[T, ID]) listHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	fmt.Printf("List %s\n", s.Name)

	// Parse query parameters
	var params FindParams
	// sort (format: column:asc|desc)
	if v := r.URL.Query().Get("sort"); v != "" {
		parts := strings.Split(v, ":")
		if len(parts) == 2 {
			params.SortBy = repo.SortBy[string]{
				Fields:    []string{parts[0]},
				Ascending: parts[1] == "asc",
			}
		}
	}
	// search
	params.Search = r.URL.Query().Get("search")
	// (Additional Filters could be appended here...)

	// Fetch data
	items, err := s.Store.List(ctx, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build table config
	tcfg := sfui.NewTableConfig(s.Name, s.Path)
	// Add columns based on schema Fields
	for _, f := range s.Fields {
		tcfg.AddCols(
			sfui.Column(f.Key(), f.Label()),
		)
	}

	// Add rows
	for _, item := range items {
		// Prepare cell components
		cells := make([]templ.Component, 0, len(s.Fields))
		rv := reflect.ValueOf(item)
		if rv.Kind() == reflect.Ptr {
			rv = rv.Elem()
		}
		for _, f := range s.Fields {
			fv := rv.FieldByNameFunc(func(name string) bool {
				return strings.EqualFold(name, f.Key())
			})
			val := ""
			if fv.IsValid() {
				val = fmt.Sprint(fv.Interface())
			}
			cells = append(cells, templ.Raw(val))
		}
		// Construct drawer URL for edit
		idVal := reflect.ValueOf(item)
		if idVal.Kind() == reflect.Ptr {
			idVal = idVal.Elem()
		}
		idField := idVal.FieldByName(s.IDField)
		url := fmt.Sprintf("%s/%v/edit", s.Path, idField.Interface())

		tcfg.AddRows(
			sfui.Row(cells...).ApplyOpts(sfui.WithDrawer(url)),
		)
	}

	// Render table or rows for HTMX
	if htmx.IsHxRequest(r) {
		templ.Handler(sfui.Rows(tcfg)).ServeHTTP(w, r)
	} else {
		templ.Handler(sfui.Page(tcfg)).ServeHTTP(w, r)
	}
}

func (s *Schema[T, ID]) newHandler(w http.ResponseWriter, r *http.Request) {
	cfg := formui.NewFormConfig("New Currency", s.Path, s.Path, "Create").Add(s.Fields...)
	templ.Handler(formui.Page(cfg), templ.WithStreaming()).ServeHTTP(w, r)
}

func (s *Schema[T, ID]) createHandler(w http.ResponseWriter, r *http.Request) {

}

func (s *Schema[T, ID]) editHandler(w http.ResponseWriter, r *http.Request) {}

func (s *Schema[T, ID]) updateHandler(w http.ResponseWriter, r *http.Request) {}

func (s *Schema[T, ID]) deleteHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := mux.Vars(r)["id"]

	// parse the string into ID
	idVal, err := parseID[ID](idStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid ID: %v", err), http.StatusBadRequest)
		return
	}

	// call into the store with the correctly-typed ID
	if err := s.Store.Delete(ctx, idVal); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, s.Path, http.StatusSeeOther)
}
