package services

import (
	"context"
	"github.com/iota-agency/iota-sdk/internal/domain/aggregates/user"
	"github.com/iota-agency/iota-sdk/pkg/event"
)

type UserService struct {
	repo      user.Repository
	publisher event.Publisher
}

func NewUserService(repo user.Repository, publisher event.Publisher) *UserService {
	return &UserService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	return s.repo.GetByEmail(ctx, email)
}

func (s *UserService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *UserService) GetAll(ctx context.Context) ([]*user.User, error) {
	return s.repo.GetAll(ctx)
}

func (s *UserService) GetByID(ctx context.Context, id uint) (*user.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*user.User, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *UserService) Create(ctx context.Context, data *user.User) error {
	createdEvent, err := user.NewCreatedEvent(ctx, *data)
	if err != nil {
		return err
	}
	if err := data.SetPassword(data.Password); err != nil {
		return err
	}
	if err := s.repo.Create(ctx, data); err != nil {
		return err
	}
	createdEvent.Result = *data
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *UserService) UpdateLastAction(ctx context.Context, id uint) error {
	return s.repo.UpdateLastAction(ctx, id)
}

func (s *UserService) UpdateLastLogin(ctx context.Context, id uint) error {
	return s.repo.UpdateLastLogin(ctx, id)
}

func (s *UserService) Update(ctx context.Context, data *user.User) error {
	updatedEvent, err := user.NewUpdatedEvent(ctx, *data)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	updatedEvent.Result = *data
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *UserService) Delete(ctx context.Context, id uint) (*user.User, error) {
	deletedEvent, err := user.NewDeletedEvent(ctx)
	if err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent.Result = *entity
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
