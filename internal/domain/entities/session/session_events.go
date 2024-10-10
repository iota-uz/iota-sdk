package session

func NewCreatedEvent(data CreateDTO, result Session) (*CreatedEvent, error) {
	return &CreatedEvent{
		Data:   data,
		Result: result,
	}, nil
}

func NewDeletedEvent(result Session) (*DeletedEvent, error) {
	return &DeletedEvent{
		Result: result,
	}, nil
}

type CreatedEvent struct {
	Data   CreateDTO
	Result Session
}

type DeletedEvent struct {
	Result Session
}
