package order

type CreateDTO struct {
	Type   string
	Status string
	// NOTE: ProductIDs should be handled by service layer
	// Service will fetch actual entities from repositories and add items
	ProductIDs []uint
}

type UpdateDTO struct {
	Type   string
	Status string
	// NOTE: ProductIDs should be handled by service layer
	// Service will fetch actual entities from repositories and add items
	ProductIDs []uint
}

func (d *CreateDTO) ToEntity() (Order, error) {
	t, err := NewType(d.Type)
	if err != nil {
		return nil, err
	}
	s, err := NewStatus(d.Status)
	if err != nil {
		return nil, err
	}
	// Return basic order without items
	// Service layer will handle AddItem with real entities fetched from repositories
	return New(t, WithStatus(s)), nil
}

func (d *UpdateDTO) ToEntity(id uint) (Order, error) {
	t, err := NewType(d.Type)
	if err != nil {
		return nil, err
	}
	s, err := NewStatus(d.Status)
	if err != nil {
		return nil, err
	}
	// Return basic order without items
	// Service layer will handle AddItem with real entities fetched from repositories
	return New(t, WithID(id), WithStatus(s)), nil
}
