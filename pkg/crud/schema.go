package crud

import (
	"context"
	"encoding"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	formui "github.com/iota-uz/iota-sdk/components/scaffold/form"
	sfui "github.com/iota-uz/iota-sdk/components/scaffold/table"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

// ErrNotFound is returned when an entity is not found
var ErrNotFound = errors.New("entity not found")

// ErrValidation is returned for validation errors
var ErrValidation = errors.New("validation error")

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

// FieldError represents a single field validation error
type FieldError struct {
	Field   string
	Message string
}

// ValidationError collects field-level validation errors
type ValidationError struct {
	Errors []FieldError
}

func (ve ValidationError) Error() string {
	if len(ve.Errors) == 0 {
		return "no validation errors"
	}

	var sb strings.Builder
	sb.WriteString("validation errors: ")
	for i, err := range ve.Errors {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%s: %s", err.Field, err.Message))
	}
	return sb.String()
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

// DefaultEntityFactory is the default implementation of EntityFactory
type DefaultEntityFactory[T any] struct{}

// Create instantiates a new entity of type T
func (f DefaultEntityFactory[T]) Create() T {
	// Create a new entity using reflection
	entityType := reflect.TypeOf(*new(T))
	if entityType.Kind() == reflect.Ptr {
		entityType = entityType.Elem()
	}
	return reflect.New(entityType).Interface().(T)
}

// EntityPatcher applies form values to an entity
type EntityPatcher[T any] interface {
	Patch(entity T, formData map[string]string, fields []formui.Field) (T, ValidationError)
}

// DefaultEntityPatcher is the default implementation of EntityPatcher
type DefaultEntityPatcher[T any] struct{}

// Patch applies form values to the entity
func (p DefaultEntityPatcher[T]) Patch(entity T, formData map[string]string, fields []formui.Field) (T, ValidationError) {
	var validationErrors ValidationError

	// Populate fields from form data
	rVal := reflect.ValueOf(entity)
	if rVal.Kind() == reflect.Ptr {
		rVal = rVal.Elem()
	}

	// Process each field
	for _, field := range fields {
		fieldName := field.Key()
		formValue, exists := formData[fieldName]
		if !exists {
			continue
		}

		// Get the field by name (case-insensitive)
		fv := rVal.FieldByNameFunc(func(name string) bool {
			return strings.EqualFold(name, fieldName)
		})

		if !fv.IsValid() || !fv.CanSet() {
			continue
		}

		// Set field value based on type
		if err := setFieldValue(fv, formValue); err != nil {
			validationErrors.Errors = append(validationErrors.Errors, FieldError{
				Field:   fieldName,
				Message: err.Error(),
			})
		}
	}

	return entity, validationErrors
}

// Helper function to set field value based on type
func setFieldValue(field reflect.Value, value string) error {
	if value == "" {
		return nil // Skip empty values
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v, err := strconv.ParseInt(value, 10, 64); err != nil {
			return fmt.Errorf("invalid integer: %w", err)
		} else {
			field.SetInt(v)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v, err := strconv.ParseUint(value, 10, 64); err != nil {
			return fmt.Errorf("invalid unsigned integer: %w", err)
		} else {
			field.SetUint(v)
		}
	case reflect.Float32, reflect.Float64:
		if v, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("invalid float: %w", err)
		} else {
			field.SetFloat(v)
		}
	case reflect.Bool:
		if v, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("invalid boolean: %w", err)
		} else {
			field.SetBool(v)
		}
	case reflect.Struct:
		// Handle common struct types
		if field.Type() == reflect.TypeOf(time.Time{}) {
			if t, err := time.Parse(time.RFC3339, value); err != nil {
				return fmt.Errorf("invalid time format: %w", err)
			} else {
				field.Set(reflect.ValueOf(t))
			}
		} else {
			return fmt.Errorf("unsupported struct type: %v", field.Type())
		}
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Array, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice, reflect.UnsafePointer:
		return fmt.Errorf("unsupported field type: %v", field.Kind())
	default:
		return fmt.Errorf("unsupported field type: %v", field.Kind())
	}
	return nil
}

// Service encapsulates the business logic of the CRUD operations
type Service[T any, ID any] struct {
	Name            string
	Path            string
	IDField         string
	Fields          []formui.Field
	Store           DataStore[T, ID]
	EntityFactory   EntityFactory[T]
	EntityPatcher   EntityPatcher[T]
	ModelValidators []ModelLevelValidator[T]
}

// NewService creates a new CRUD service
func NewService[T any, ID any](
	name, path, idField string,
	store DataStore[T, ID],
	fields []formui.Field,
) *Service[T, ID] {
	return &Service[T, ID]{
		Name:          name,
		Path:          path,
		IDField:       idField,
		Fields:        fields,
		Store:         store,
		EntityFactory: DefaultEntityFactory[T]{},
		EntityPatcher: DefaultEntityPatcher[T]{},
	}
}

// List retrieves entities based on the provided parameters
func (s *Service[T, ID]) List(ctx context.Context, params FindParams) ([]T, error) {
	return s.Store.List(ctx, params)
}

// Get retrieves a single entity by ID
func (s *Service[T, ID]) Get(ctx context.Context, id ID) (T, error) {
	return s.Store.Get(ctx, id)
}

// Extract returns entity field values as map for UI rendering
func (s *Service[T, ID]) Extract(entity T) map[string]string {
	result := make(map[string]string)
	rVal := reflect.ValueOf(entity)
	if rVal.Kind() == reflect.Ptr {
		rVal = rVal.Elem()
	}

	for _, field := range s.Fields {
		fieldName := field.Key()
		fv := rVal.FieldByNameFunc(func(name string) bool {
			return strings.EqualFold(name, fieldName)
		})

		if fv.IsValid() {
			// Convert the field value to string
			var strVal string
			if fv.Kind() == reflect.String {
				strVal = fv.String()
			} else {
				strVal = fmt.Sprint(fv.Interface())
			}
			result[fieldName] = strVal
		}
	}

	return result
}

// CreateEntity creates a new entity from form data
func (s *Service[T, ID]) CreateEntity(ctx context.Context, formData map[string]string) (ID, error) {
	// Create a new entity
	entity := s.EntityFactory.Create()

	// Apply form data to entity
	patchedEntity, valErrs := s.EntityPatcher.Patch(entity, formData, s.Fields)
	if len(valErrs.Errors) > 0 {
		return *new(ID), fmt.Errorf("%w: %s", ErrValidation, valErrs.Error())
	}

	// Run model-level validation
	for _, validator := range s.ModelValidators {
		if err := validator.ValidateModel(ctx, patchedEntity); err != nil {
			return *new(ID), fmt.Errorf("%w: %s", ErrValidation, err.Error())
		}
	}

	// Create the entity
	id, err := s.Store.Create(ctx, patchedEntity)
	if err != nil {
		return *new(ID), err
	}

	return id, nil
}

// UpdateEntity updates an existing entity from form data
func (s *Service[T, ID]) UpdateEntity(ctx context.Context, id ID, formData map[string]string) error {
	// Get the existing entity first
	entity, err := s.Store.Get(ctx, id)
	if err != nil {
		return err
	}

	// Apply form data to entity
	patchedEntity, valErrs := s.EntityPatcher.Patch(entity, formData, s.Fields)
	if len(valErrs.Errors) > 0 {
		return fmt.Errorf("%w: %s", ErrValidation, valErrs.Error())
	}

	// Run model-level validation
	for _, validator := range s.ModelValidators {
		if err := validator.ValidateModel(ctx, patchedEntity); err != nil {
			return fmt.Errorf("%w: %s", ErrValidation, err.Error())
		}
	}

	// Update the entity
	return s.Store.Update(ctx, id, patchedEntity)
}

// DeleteEntity deletes an entity by ID
func (s *Service[T, ID]) DeleteEntity(ctx context.Context, id ID) error {
	return s.Store.Delete(ctx, id)
}

// Schema defines a runtime-driven CRUD resource for entity type T with identifier type ID.
type Schema[T any, ID any] struct {
	Service       *Service[T, ID]
	Renderer      RenderFunc
	middlewares   []mux.MiddlewareFunc
	getPrimaryKey func() string
}

// Ensure Schema implements application.Controller
var _ application.Controller = (*Schema[any, any])(nil)

// SchemaOpt configures optional settings on a Schema
type SchemaOpt[T any, ID any] func(s *Schema[T, ID])

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

// WithFields sets the form fields for the Schema
func WithFields[T any, ID any](fields ...formui.Field) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.Service.Fields = fields
	}
}

// WithModelValidators adds model-level validators
func WithModelValidators[T any, ID any](vs ...ModelLevelValidator[T]) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.Service.ModelValidators = vs
	}
}

// WithMiddlewares adds middleware functions to the Schema
func WithMiddlewares[T any, ID any](ms ...mux.MiddlewareFunc) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.middlewares = append(s.middlewares, ms...)
	}
}

// WithRenderer sets a custom renderer function
func WithRenderer[T any, ID any](renderer RenderFunc) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.Renderer = renderer
	}
}

// WithGetPrimaryKey sets a custom function to get the primary key field name
func WithGetPrimaryKey[T any, ID any](fn func() string) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.getPrimaryKey = fn
	}
}

// WithEntityFactory sets a custom entity factory
func WithEntityFactory[T any, ID any](factory EntityFactory[T]) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.Service.EntityFactory = factory
	}
}

// WithEntityPatcher sets a custom entity patcher
func WithEntityPatcher[T any, ID any](patcher EntityPatcher[T]) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.Service.EntityPatcher = patcher
	}
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

// parseFormToMap extracts form values into a map
func parseFormToMap(r *http.Request) (map[string]string, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for key, values := range r.Form {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}

	return result, nil
}

// HTTP Handlers
func (s *Schema[T, ID]) listHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

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
	items, err := s.Service.List(ctx, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build table config
	tcfg := sfui.NewTableConfig(s.Service.Name, s.Service.Path)
	// Add columns based on schema Fields
	for _, f := range s.Service.Fields {
		tcfg.AddCols(
			sfui.Column(f.Key(), f.Label()),
		)
	}

	// Add rows
	for _, item := range items {
		// Prepare cell components
		cells := make([]templ.Component, 0, len(s.Service.Fields))
		rv := reflect.ValueOf(item)
		if rv.Kind() == reflect.Ptr {
			rv = rv.Elem()
		}
		for _, f := range s.Service.Fields {
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
		idField := idVal.FieldByName(s.Service.IDField)
		url := fmt.Sprintf("%s/%v/edit", s.Service.Path, idField.Interface())

		tcfg.AddRows(
			sfui.Row(cells...).ApplyOpts(sfui.WithDrawer(url)),
		)
	}

	// Render table or rows for HTMX
	if htmx.IsHxRequest(r) {
		s.Renderer(w, r, sfui.Rows(tcfg))
	} else {
		s.Renderer(w, r, sfui.Page(tcfg))
	}
}

func (s *Schema[T, ID]) newHandler(w http.ResponseWriter, r *http.Request) {
	cfg := formui.NewFormConfig("New "+s.Service.Name, s.Service.Path, s.Service.Path, "Create").
		Add(s.Service.Fields...)
	s.Renderer(w, r, formui.Page(cfg), templ.WithStreaming())
}

func (s *Schema[T, ID]) createHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	formData, err := parseFormToMap(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = s.Service.CreateEntity(ctx, formData)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Redirect to the list view
	http.Redirect(w, r, s.Service.Path, http.StatusSeeOther)
}

func (s *Schema[T, ID]) editHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := mux.Vars(r)["id"]

	// Parse the ID
	idVal, err := parseID[ID](idStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid ID: %v", err), http.StatusBadRequest)
		return
	}

	// Fetch the entity
	entity, err := s.Service.Get(ctx, idVal)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	// Extract field values from entity
	fieldValues := s.Service.Extract(entity)

	// Create fields with values
	fields := make([]formui.Field, len(s.Service.Fields))
	for i, field := range s.Service.Fields {
		switch f := field.(type) {
		case formui.TextField:
			if val, ok := fieldValues[field.Key()]; ok {
				fields[i] = f.WithValue(val)
			} else {
				fields[i] = field
			}
		default:
			fields[i] = field
		}
	}

	// Create form with entity values
	formAction := fmt.Sprintf("%s/%v", s.Service.Path, idVal)
	cfg := formui.NewFormConfig("Edit "+s.Service.Name, formAction, s.Service.Path, "Update").
		WithMethod("PUT").
		Add(fields...)

	s.Renderer(w, r, formui.Page(cfg), templ.WithStreaming())
}

func (s *Schema[T, ID]) updateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := mux.Vars(r)["id"]

	// Parse the ID
	idVal, err := parseID[ID](idStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid ID: %v", err), http.StatusBadRequest)
		return
	}

	formData, err := parseFormToMap(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = s.Service.UpdateEntity(ctx, idVal, formData)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrValidation) {
			status = http.StatusBadRequest
		} else if errors.Is(err, ErrNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	// Redirect to the list view
	http.Redirect(w, r, s.Service.Path, http.StatusSeeOther)
}

func (s *Schema[T, ID]) deleteHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := mux.Vars(r)["id"]

	// parse the string into ID
	idVal, err := parseID[ID](idStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid ID: %v", err), http.StatusBadRequest)
		return
	}

	// call into the service with the correctly-typed ID
	err = s.Service.DeleteEntity(ctx, idVal)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	http.Redirect(w, r, s.Service.Path, http.StatusSeeOther)
}
