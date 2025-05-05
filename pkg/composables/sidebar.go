package composables

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

var (
	ErrNavItemsNotFound = errors.New("navigation items not found")
)

func UseNavItems(ctx context.Context) []types.NavigationItem {
	navItems := ctx.Value(constants.NavItemsKey)
	if navItems == nil {
		panic(ErrNavItemsNotFound)
	}
	return navItems.([]types.NavigationItem)
}

func UseAllNavItems(ctx context.Context) ([]types.NavigationItem, error) {
	navItems := ctx.Value(constants.AllNavItemsKey)
	if navItems == nil {
		return nil, ErrNavItemsNotFound
	}
	return navItems.([]types.NavigationItem), nil
}
