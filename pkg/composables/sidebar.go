package composables

import (
	"errors"
	"github.com/iota-agency/iota-sdk/internal/types"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"net/http"
)

var (
	ErrNavItemsNotFound = errors.New("navigation items not found")
)

func UseNavItems(r *http.Request) ([]types.NavigationItem, error) {
	navItems := r.Context().Value(constants.NavItemsKey)
	if navItems == nil {
		return nil, ErrNavItemsNotFound
	}
	return navItems.([]types.NavigationItem), nil
}
