package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen version v0.17.57

import (
	"context"

	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	model "github.com/iota-agency/iota-sdk/modules/warehouse/interfaces/graph/gqlmodels"
	"github.com/iota-agency/iota-sdk/modules/warehouse/interfaces/graph/mappers"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
)

// WarehousePosition is the resolver for the warehousePosition field.
func (r *queryResolver) WarehousePosition(ctx context.Context, id int64) (*model.WarehousePosition, error) {
	domainPosition, err := r.positionService.GetByID(ctx, uint(id))
	if err != nil {
		return nil, err
	}
	return mappers.PositionToGraphModel(domainPosition), nil
}

// WarehousePositions is the resolver for the warehousePositions field.
func (r *queryResolver) WarehousePositions(ctx context.Context, offset int, limit int, sortBy []string) (*model.PaginatedWarehousePositions, error) {
	domainPositions, err := r.positionService.GetPaginated(ctx, &position.FindParams{
		Offset: offset,
		Limit:  limit,
		SortBy: sortBy,
	})
	if err != nil {
		return nil, err
	}
	total, err := r.positionService.Count(ctx)
	if err != nil {
		return nil, err
	}
	return &model.PaginatedWarehousePositions{
		Data:  mapping.MapViewModels(domainPositions, mappers.PositionToGraphModel),
		Total: total,
	}, nil
}