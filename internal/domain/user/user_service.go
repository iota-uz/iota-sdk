package user

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/infrastracture/event"
)

type Service struct {
	repo           Repository
	eventPublisher *event.Publisher
}

func NewUserService(repo Repository, eventPublisher *event.Publisher) *Service {
	return &Service{
		repo:           repo,
		eventPublisher: eventPublisher,
	}
}

func (s *Service) GetUsersCount(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *Service) GetUsers(ctx context.Context) ([]*User, error) {
	return s.repo.GetAll(ctx)
}

func (s *Service) GetUserByID(ctx context.Context, id int64) (*User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) GetUsersPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*User, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *Service) CreateUser(ctx context.Context, user *User) error {
	if err := s.repo.Create(ctx, user); err != nil {
		return err
	}
	s.eventPublisher.Publish("user.created", user)
	return nil
}

func (s *Service) UpdateUser(ctx context.Context, user *User) error {
	if err := s.repo.Update(ctx, user); err != nil {
		return err
	}
	s.eventPublisher.Publish("user.updated", user)
	return nil
}

func (s *Service) DeleteUser(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.eventPublisher.Publish("user.deleted", id)
	return nil
}
