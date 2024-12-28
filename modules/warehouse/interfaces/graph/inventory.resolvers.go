package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen version v0.17.57

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/inventory"
	model "github.com/iota-agency/iota-sdk/modules/warehouse/interfaces/graph/gqlmodels"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/serrors"
)

// CompleteInventoryCheck is the resolver for the completeInventoryCheck field.
func (r *mutationResolver) CompleteInventoryCheck(ctx context.Context, items []*model.InventoryItem) (bool, error) {
	_, err := composables.UseUser(ctx)
	if err != nil {
		graphql.AddError(ctx, serrors.UnauthorizedGQLError(graphql.GetPath(ctx)))
		return false, nil
	}
	dto := &inventory.CreateCheckDTO{
		Name:      "Inventory check",
		Positions: make([]*inventory.PositionCheckDTO, 0, len(items)),
	}
	for _, item := range items {
		dto.Positions = append(dto.Positions, &inventory.PositionCheckDTO{
			PositionID: uint(item.PositionID),
			Found:      uint(item.Found),
		})
	}
	if _, err := r.inventoryService.Create(ctx, dto); err != nil {
		return false, err
	}
	return true, nil
}

// Inventory is the resolver for the inventory field.
func (r *queryResolver) Inventory(ctx context.Context) ([]*model.InventoryPosition, error) {
	_, err := composables.UseUser(ctx)
	if err != nil {
		graphql.AddError(ctx, serrors.UnauthorizedGQLError(graphql.GetPath(ctx)))
		return nil, nil
	}
	positions, err := r.inventoryService.Positions(ctx)
	if err != nil {
		return nil, err
	}
	return InventoryPositionsToGraphModel(positions), nil
}

// Mutation returns MutationResolver implementation.
func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }

type mutationResolver struct{ *Resolver }
