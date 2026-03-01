// Package session provides this package.
package session

func NewCreatedEvent(data Session) (*CreatedEvent, error) {
	return &CreatedEvent{
		Result: data,
	}, nil
}

func NewUpdatedEvent(data Session) (*UpdatedEvent, error) {
	return &UpdatedEvent{
		Data:   data,
		Result: data,
	}, nil
}

func NewDeletedEvent(result Session) (*DeletedEvent, error) {
	return &DeletedEvent{
		Result: result,
	}, nil
}

type CreatedEvent struct {
	Result Session
}

type UpdatedEvent struct {
	Data   Session
	Result Session
}

type DeletedEvent struct {
	Result Session
}
