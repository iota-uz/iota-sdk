package composables

import (
	"context"
	"errors"

	"github.com/iota-agency/iota-sdk/pkg/constants"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/tab"
)

var (
	ErrTabsNotFound = errors.New("no tabs found")
)

func UseTabs(ctx context.Context) ([]*tab.Tab, error) {
	tabs, ok := ctx.Value(constants.TabsKey).([]*tab.Tab)
	if !ok {
		return make([]*tab.Tab, 0), ErrTabsNotFound
	}
	return tabs, nil
}
