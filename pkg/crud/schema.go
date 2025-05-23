package crud

import (
	"net/http"

	"github.com/gorilla/mux"
	formui "github.com/iota-uz/iota-sdk/components/scaffold/form"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

// Schema defines a runtime-driven CRUD resource for entity type T with identifier type ID.
type Schema[T any, ID any] struct {
	Service       *Service[T, ID]
	Renderer      RenderFunc
	middlewares   []mux.MiddlewareFunc
	getPrimaryKey func() string
}

// Ensure Schema implements application.Controller
var _ application.Controller = (*Schema[any, any])(nil)

// DefaultGetPrimaryKey returns a function that gets the primary key
func DefaultGetPrimaryKey[T any]() func() string {
	return func() string {
		// Default implementation - replace with actual logic to determine primary key
		return "ID"
	}
}

// NewSchema constructs a new CRUD Schema and applies options
func NewSchema[T any, ID any](
	name, path string,
	store DataStore[T, ID],
	opts ...SchemaOpt[T, ID],
) *Schema[T, ID] {
	service := NewService[T, ID](
		name,
		path,
		"ID", // Default ID field, can be overridden with options
		store,
		[]formui.Field{}, // Empty fields, can be added with options
	)

	s := &Schema[T, ID]{
		Service:       service,
		Renderer:      DefaultRenderFunc,
		getPrimaryKey: DefaultGetPrimaryKey[T](),
	}

	for _, o := range opts {
		o(s)
	}

	// Update the IDField from the getPrimaryKey function
	s.Service.IDField = s.getPrimaryKey()

	return s
}

// Register mounts CRUD HTTP handlers on the provided router
func (s *Schema[T, ID]) Register(r *mux.Router) {
	subR := r.PathPrefix(s.Service.Path).Subrouter()
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
	return s.Service.Path
}
