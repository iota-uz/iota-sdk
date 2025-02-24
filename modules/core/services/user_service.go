package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type UserService struct {
	repo      user.Repository
	publisher eventbus.EventBus
}

func NewUserService(repo user.Repository, publisher eventbus.EventBus) *UserService {
	return &UserService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (user.User, error) {
	return s.repo.GetByEmail(ctx, email)
}

func (s *UserService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *UserService) GetAll(ctx context.Context) ([]user.User, error) {
	return s.repo.GetAll(ctx)
}

func (s *UserService) GetByID(ctx context.Context, id uint) (user.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) GetPaginated(ctx context.Context, params *user.FindParams) ([]user.User, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *UserService) Create(ctx context.Context, data user.User) error {
	data, err := data.SetPassword(data.Password())
	if err != nil {
		return err
	}
	if _, err := s.repo.Create(ctx, data); err != nil {
		return err
	}
	return nil
}

func (s *UserService) UpdateLastAction(ctx context.Context, id user.UserID) error {
	return s.repo.UpdateLastAction(ctx, id)
}

func (s *UserService) UpdateLastLogin(ctx context.Context, id user.UserID) error {
	return s.repo.UpdateLastLogin(ctx, id)
}

func (s *UserService) Update(ctx context.Context, data user.User) error {
	var err error
	if data.Password() != "" {
		data, err = data.SetPassword(data.Password())
		if err != nil {
			return err
		}
	}
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	return nil
}

func (s *UserService) Delete(ctx context.Context, id uint) (user.User, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	return entity, nil
}
