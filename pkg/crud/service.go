package crud

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type Service[TEntity any] interface {
	GetAll(ctx context.Context) ([]TEntity, error)
	Get(ctx context.Context, value FieldValue) (TEntity, error)
	Exists(ctx context.Context, value FieldValue) (bool, error)
	Count(ctx context.Context, params *FindParams) (int64, error)
	List(ctx context.Context, params *FindParams) ([]TEntity, error)
	Save(ctx context.Context, entity TEntity) (TEntity, error)
	Delete(ctx context.Context, value FieldValue) (TEntity, error)
}

func DefaultService[TEntity any](
	schema Schema[TEntity],
	repository Repository[TEntity],
	publisher eventbus.EventBus,
) Service[TEntity] {
	return &service[TEntity]{
		schema:     schema,
		repository: repository,
		publisher:  publisher,
	}
}

type service[TEntity any] struct {
	schema     Schema[TEntity]
	repository Repository[TEntity]
	publisher  eventbus.EventBus
}

func (s *service[TEntity]) GetAll(ctx context.Context) ([]TEntity, error) {
	entities, err := s.repository.GetAll(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "service failed to get all entities")
	}
	return entities, nil
}

func (s *service[TEntity]) Get(ctx context.Context, value FieldValue) (TEntity, error) {
	entity, err := s.repository.Get(ctx, value)
	if err != nil {
		return entity, errors.Wrap(err, "service failed to get entity by value")
	}
	return entity, nil
}

func (s *service[TEntity]) Exists(ctx context.Context, value FieldValue) (bool, error) {
	exists, err := s.repository.Exists(ctx, value)
	if err != nil {
		return false, errors.Wrap(err, "service failed to check existence")
	}
	return exists, nil
}

func (s *service[TEntity]) Count(ctx context.Context, params *FindParams) (int64, error) {
	count, err := s.repository.Count(ctx, params)
	if err != nil {
		return 0, errors.Wrap(err, "service failed to count entities")
	}
	return count, nil
}

func (s *service[TEntity]) List(ctx context.Context, params *FindParams) ([]TEntity, error) {
	entities, err := s.repository.List(ctx, params)
	if err != nil {
		return nil, errors.Wrap(err, "service failed to list entities")
	}
	return entities, nil
}

func (s *service[TEntity]) Save(ctx context.Context, entity TEntity) (TEntity, error) {
	var zero TEntity

	fieldValues, err := s.schema.Mapper().ToFieldValues(ctx, entity)
	if err != nil {
		return zero, errors.Wrap(err, "failed to map entity to field values for saving")
	}

	var keyFieldVal FieldValue
	for _, fv := range fieldValues {
		if fv.Field().Key() {
			keyFieldVal = fv
			break
		}
	}
	if keyFieldVal == nil {
		return zero, errors.New("missing primary key in entity for save operation")
	}

	isCreate := keyFieldVal.IsZero()

	if !isCreate {
		exists, err := s.repository.Exists(ctx, keyFieldVal)
		if err != nil {
			return zero, errors.Wrap(err, "service failed to check if entity exists")
		}
		if exists {
			isCreate = false
		}
	}

	var (
		savedEntity TEntity
		event       Event[TEntity]
	)

	if isCreate {
		if event, err = NewCreatedEvent(ctx, entity); err != nil {
			return zero, errors.Wrap(err, "failed to create 'created' event")
		}
	} else {
		if event, err = NewUpdatedEvent(ctx, entity); err != nil {
			return zero, errors.Wrap(err, "failed to create 'updated' event")
		}
	}

	if err := composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.validation(txCtx, entity); err != nil {
			return errors.Wrap(err, "entity validation failed")
		}
		if isCreate {
			savedEntity, err = s.repository.Create(txCtx, fieldValues)
			if err != nil {
				return errors.Wrap(err, "failed to create entity in repository")
			}
		} else {
			savedEntity, err = s.repository.Update(txCtx, fieldValues)
			if err != nil {
				return errors.Wrap(err, "failed to update entity in repository")
			}
		}
		return nil
	}); err != nil {
		return zero, errors.Wrap(err, "transaction failed during save operation")
	}

	event.SetResult(savedEntity)
	s.publisher.Publish(event)

	return savedEntity, nil
}

func (s *service[TEntity]) Delete(ctx context.Context, value FieldValue) (TEntity, error) {
	var zero TEntity

	deletedEvent, err := NewDeletedEvent[TEntity](ctx)

	var deletedEntity TEntity
	if err := composables.InTx(ctx, func(txCtx context.Context) error {
		if entity, err := s.repository.Delete(txCtx, value); err != nil {
			return errors.Wrap(err, "failed to delete entity")
		} else {
			deletedEntity = entity
		}
		return nil
	}); err != nil {
		return zero, errors.Wrap(err, "transaction failed during save operation")
	}
	if err != nil {
		return zero, errors.Wrap(err, "transaction failed during delete operation")
	}

	deletedEvent.Data = deletedEntity
	s.publisher.Publish(deletedEvent)

	return deletedEntity, nil
}

func (s *service[TEntity]) validation(ctx context.Context, entity TEntity) error {
	var errs []error

	fieldValues, err := s.schema.Mapper().ToFieldValues(ctx, entity)
	if err != nil {
		errs = append(errs, errors.Wrap(err, "failed to map entity to field values for validation"))
		return errors.Join(errs...)
	}

	var keyFieldVal FieldValue
	readonlyFieldValues := make([]FieldValue, 0)
	for _, fv := range fieldValues {
		if fv.Field().Key() {
			keyFieldVal = fv
		}

		if fv.Field().Readonly() {
			readonlyFieldValues = append(readonlyFieldValues, fv)
		}
		for _, rule := range fv.Field().Rules() {
			if ruleErr := rule(fv); ruleErr != nil {
				errs = append(errs, errors.Wrap(ruleErr, fmt.Sprintf("validation rule failed for field %q", fv.Field().Name())))
			}
		}
	}
	if keyFieldVal == nil {
		errs = append(errs, errors.New("missing primary key for validation"))
		return errors.Join(errs...)
	}

	if !keyFieldVal.IsZero() && len(readonlyFieldValues) > 0 {
		dbEntity, err := s.repository.Get(ctx, keyFieldVal)
		if err != nil {
			errs = append(errs, errors.Wrap(err, "failed to retrieve existing entity for readonly field validation"))
			return errors.Join(errs...)
		}

		dbFieldValues, err := s.schema.Mapper().ToFieldValues(ctx, dbEntity)
		if err != nil {
			errs = append(errs, errors.Wrap(err, "failed to map database entity to field values for readonly validation"))
			return errors.Join(errs...)
		}

		dbReadonlyMap := make(map[string]FieldValue, len(readonlyFieldValues))
		for _, dbFv := range dbFieldValues {
			if dbFv.Field().Readonly() {
				dbReadonlyMap[dbFv.Field().Name()] = dbFv
			}
		}

		for _, readonlyFv := range readonlyFieldValues {
			if dbFv, ok := dbReadonlyMap[readonlyFv.Field().Name()]; ok {
				if readonlyFv.Value() != dbFv.Value() {
					errs = append(errs, errors.Errorf("readonly field %q has been modified", readonlyFv.Field().Name()))
				}
			}
		}
	}

	for _, v := range s.schema.Validators() {
		if valErr := v(entity); valErr != nil {
			errs = append(errs, errors.Wrap(valErr, "schema-level validation failed"))
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return errors.Join(errs...)
}
