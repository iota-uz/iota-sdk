package crud

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	formui "github.com/iota-uz/iota-sdk/components/scaffold/form"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

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

// RenderFunc abstracts the rendering logic to make it testable
type RenderFunc func(w http.ResponseWriter, r *http.Request, component templ.Component, options ...func(*templ.ComponentHandler))

// DefaultRenderFunc provides the default rendering implementation
func DefaultRenderFunc(w http.ResponseWriter, r *http.Request, component templ.Component, options ...func(*templ.ComponentHandler)) {
	templ.Handler(component, options...).ServeHTTP(w, r)
}

// EntityFactory creates new instances of entity type T
type EntityFactory[T any] interface {
	Create() T
}

// EntityPatcher applies form values to an entity
type EntityPatcher[T any] interface {
	Patch(entity T, formData map[string]string, fields []formui.Field) (T, ValidationError)
}

// SchemaOpt configures optional settings on a Schema
type SchemaOpt[T any, ID any] func(s *Schema[T, ID])
