// Package services provides this package.
package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type UserQueryService struct {
	repo query.UserQueryRepository
}

func NewUserQueryService(repo query.UserQueryRepository) *UserQueryService {
	return &UserQueryService{repo: repo}
}

func (s *UserQueryService) FindUsers(ctx context.Context, params *query.FindParams) ([]*viewmodels.User, int, error) {
	return s.repo.FindUsers(ctx, params)
}

func (s *UserQueryService) FindUserByID(ctx context.Context, userID int) (*viewmodels.User, error) {
	return s.repo.FindUserByID(ctx, userID)
}

func (s *UserQueryService) CanDeleteUser(ctx context.Context, userID int) (bool, error) {
	const op = serrors.Op("UserQueryService.CanDeleteUser")
	canDelete, err := s.repo.CanDeleteUser(ctx, userID)
	if err != nil {
		return false, serrors.E(op, err)
	}
	return canDelete, nil
}

func (s *UserQueryService) SearchUsers(ctx context.Context, params *query.FindParams) ([]*viewmodels.User, int, error) {
	return s.repo.SearchUsers(ctx, params)
}

func (s *UserQueryService) FindUsersWithRoles(ctx context.Context, params *query.FindParams) ([]*viewmodels.User, int, error) {
	return s.repo.FindUsersWithRoles(ctx, params)
}
