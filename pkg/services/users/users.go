package users

import (
	"context"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/sdk/graphql/helpers"
	"github.com/iota-agency/iota-erp/sdk/service"
	"gorm.io/gorm"
)

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

type Service struct {
	db *gorm.DB
}

func (s *Service) useTx(ctx context.Context) *gorm.DB {
	if params, ok := composables.UseParams(ctx); ok {
		return params.Tx
	} else {
		return s.db
	}
}

func (s *Service) GetAll(ctx context.Context, params *service.FindParams) ([]*models.User, error) {
	if params == nil {
		params = &service.FindParams{}
	}
	tx := s.useTx(ctx)
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
	tx := s.useTx(ctx)
	var count int64
	if err := tx.Model(&models.User{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Service) Create(ctx context.Context, user *models.User) error {
	tx := s.useTx(ctx)
	if err := tx.Create(user).Error; err != nil {
		return err
	}
	return nil
}
