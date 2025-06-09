package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
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

func (s *GroupQueryService) FindGroupByID(ctx context.Context, groupID string) (*viewmodels.Group, error) {
	return s.repo.FindGroupByID(ctx, groupID)
}

func (s *GroupQueryService) SearchGroups(ctx context.Context, params *query.GroupFindParams) ([]*viewmodels.Group, int, error) {
	return s.repo.SearchGroups(ctx, params)
}
