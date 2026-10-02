// Package services provides this package.
package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type GroupQueryService struct {
	repo query.GroupQueryRepository
}

func NewGroupQueryService(repo query.GroupQueryRepository) *GroupQueryService {
	return &GroupQueryService{repo: repo}
}

func (s *GroupQueryService) FindGroups(ctx context.Context, params *query.GroupFindParams) ([]*viewmodels.Group, int, error) {
	return s.repo.FindGroups(ctx, params)
}

func (s *GroupQueryService) FindAssignmentOptions(ctx context.Context) ([]*viewmodels.AssignmentOption, error) {
	const op = serrors.Op("GroupQueryService.FindAssignmentOptions")
	options, err := s.repo.FindAssignmentOptions(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return options, nil
}

func (s *GroupQueryService) FindGroupLabelsByIDs(ctx context.Context, groupIDs []uuid.UUID) ([]*viewmodels.Group, error) {
	const op = serrors.Op("GroupQueryService.FindGroupLabelsByIDs")
	groups, err := s.repo.FindGroupLabelsByIDs(ctx, groupIDs)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return groups, nil
}

func (s *GroupQueryService) FindGroupPermissionsByIDs(ctx context.Context, groupIDs []uuid.UUID) (map[uuid.UUID][]permission.Permission, error) {
	const op = serrors.Op("GroupQueryService.FindGroupPermissionsByIDs")
	result, err := s.repo.FindGroupPermissionsByIDs(ctx, groupIDs)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return result, nil
}

func (s *GroupQueryService) FindGroupByID(ctx context.Context, groupID string) (*viewmodels.Group, error) {
	return s.repo.FindGroupByID(ctx, groupID)
}

func (s *GroupQueryService) SearchGroups(ctx context.Context, params *query.GroupFindParams) ([]*viewmodels.Group, int, error) {
	return s.repo.SearchGroups(ctx, params)
}
