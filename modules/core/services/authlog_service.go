package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/authlog"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/event"
)

type AuthLogService struct {
	repo      authlog.Repository
	publisher event.Publisher
}

func NewAuthLogService(repo authlog.Repository, publisher event.Publisher) *AuthLogService {
	return &AuthLogService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *AuthLogService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *AuthLogService) GetAll(ctx context.Context) ([]*authlog.AuthenticationLog, error) {
	return s.repo.GetAll(ctx)
}

func (s *AuthLogService) GetByID(ctx context.Context, id uint) (*authlog.AuthenticationLog, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *AuthLogService) GetPaginated(
	ctx context.Context, params *authlog.FindParams,
) ([]*authlog.AuthenticationLog, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *AuthLogService) Create(ctx context.Context, data *authlog.AuthenticationLog) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if err := s.repo.Create(ctx, data); err != nil {
		return err
	}
	s.publisher.Publish("authlog.created", data)
	return tx.Commit(ctx)
}

func (s *AuthLogService) Update(ctx context.Context, data *authlog.AuthenticationLog) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	s.publisher.Publish("authlog.updated", data)
	return tx.Commit(ctx)
}

func (s *AuthLogService) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.publisher.Publish("authlog.deleted", id)
	return tx.Commit(ctx)
}
