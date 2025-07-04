package composables

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"

	"github.com/iota-uz/iota-sdk/pkg/constants"
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
