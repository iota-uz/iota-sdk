package crud_v2

import "context"

type Event[TEntity any] interface {
	SetResult(result TEntity)
}

func NewCreatedEvent[TEntity any](_ context.Context, data TEntity) (*CreatedEvent[TEntity], error) {
	return &CreatedEvent[TEntity]{
		Data: data,
	}, nil
}

func NewUpdatedEvent[TEntity any](_ context.Context, data TEntity) (*UpdatedEvent[TEntity], error) {
	return &UpdatedEvent[TEntity]{
		Data: data,
	}, nil
}

func NewDeletedEvent[TEntity any](_ context.Context) (*DeletedEvent[TEntity], error) {
	return &DeletedEvent[TEntity]{}, nil
}

type CreatedEvent[TEntity any] struct {
	Data   any
	Result any
}

func (e *CreatedEvent[TEntity]) SetResult(result TEntity) {
	e.Result = result
}

type UpdatedEvent[TEntity any] struct {
	Data   any
	Result any
}

func (e *UpdatedEvent[TEntity]) SetResult(result TEntity) {
	e.Result = result
}

type DeletedEvent[TEntity any] struct {
	Data any
}
