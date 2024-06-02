package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/user"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/service"
)

func NewUserRepository() user.Repository {
	return &GormUserRepository{}
}

type GormUserRepository struct {
}

func (g *GormUserRepository) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*user.User, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var users []*user.User
	q := tx.Limit(limit).Offset(offset)
	for _, s := range sortBy {
		q = q.Order(s)
	}
	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (g *GormUserRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&user.User{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormUserRepository) GetAll(ctx context.Context) ([]*user.User, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var users []*user.User
	if err := tx.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (g *GormUserRepository) GetByID(ctx context.Context, id int64) (*user.User, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	user := &user.User{}
	if err := tx.First(user, id).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (g *GormUserRepository) Create(ctx context.Context, user *user.User) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(user).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormUserRepository) Update(ctx context.Context, user *user.User) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Save(user).Error
}

func (g *GormUserRepository) Delete(ctx context.Context, id int64) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Delete(&user.User{}, id).Error; err != nil {
		return err
	}
	return nil
}
