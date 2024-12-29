package application

import (
	"context"
	"errors"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

var (
	ErrAppNotFound = errors.New("app not found")
)

// UseApp returns the user from the context.
// If the user is not found, the second return value will be false.
func UseApp(ctx context.Context) (Application, error) {
	app := ctx.Value(constants.AppKey)
	if app == nil {
		return nil, ErrAppNotFound
	}
	return app.(Application), nil
}
