package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
)

type RoleQueryService struct {
	repo query.RoleQueryRepository
}

func NewRoleQueryService(repo query.RoleQueryRepository) *RoleQueryService {
	return &RoleQueryService{repo: repo}
}

func (s *RoleQueryService) GetRolesWithCounts(ctx context.Context) ([]*viewmodels.Role, error) {
	return s.repo.FindRolesWithCounts(ctx)
}
