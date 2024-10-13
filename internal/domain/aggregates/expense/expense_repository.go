package expense

import "context"

type Repository interface {
	GetByID(ctx context.Context, id uint) (*Expense, error)
	GetAll(ctx context.Context) ([]*Expense, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Expense, error)
	Create(ctx context.Context, data *Expense) error
	Update(ctx context.Context, data *Expense) error
	Delete(ctx context.Context, id uint) error
}
