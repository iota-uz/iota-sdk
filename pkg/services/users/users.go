package users

import (
	"context"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/graphql/helpers"
	"github.com/iota-agency/iota-erp/sdk/service"
)

func New() *Service {
	return &Service{}
}

type Service struct {
}

func (s *Service) Get(ctx context.Context, params *service.GetParams[int64]) (*models.User, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx
	for _, join := range params.Joins {
		q = q.Joins(join)
	}
	user := &models.User{}
	if err := q.First(user, params.Id).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) GetAll(ctx context.Context, params *service.FindParams) ([]*models.User, error) {
	if params == nil {
		params = &service.FindParams{}
	}
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Offset(params.Offset)
	if params.Limit > 0 {
		q = q.Limit(params.Limit)
	}
	if len(params.Joins) > 0 {
		for _, join := range params.Joins {
			q = q.Joins(join)
		}
	}
	if len(params.SortBy) > 0 {
		if res, err := helpers.ApplySort(q, params.SortBy, &models.User{}); err != nil {
			return nil, err
		} else {
			q = res
		}
	}

	var users []*models.User
	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (s *Service) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.User{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Service) Create(ctx context.Context, user *models.User) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(user).Error; err != nil {
		return err
	}
	return nil
}

func (s *Service) Update(ctx context.Context, user *models.User) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Save(user).Error
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Delete(&models.User{}, id).Error; err != nil {
		return err
	}
	return nil
}
