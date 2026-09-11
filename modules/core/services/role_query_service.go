// Package services provides this package.
package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
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

func (s *RoleQueryService) FindAssignmentOptions(ctx context.Context) ([]*viewmodels.AssignmentOption, error) {
	const op = serrors.Op("RoleQueryService.FindAssignmentOptions")
	options, err := s.repo.FindAssignmentOptions(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return options, nil
}
