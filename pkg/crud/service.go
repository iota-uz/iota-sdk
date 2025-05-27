package crud

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	formui "github.com/iota-uz/iota-sdk/components/scaffold/form"
)

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
