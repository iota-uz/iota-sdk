package composition

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/constants"
)

// UseContainer extracts the composition.Container from a request context.
// Used by middleware and request-scoped code that needs to resolve services
// per request. The server installs a middleware that places the container
// under constants.ContainerKey.
func UseContainer(ctx context.Context) (*Container, error) {
	if ctx == nil {
		return nil, fmt.Errorf("composition: context is nil")
	}
	container, ok := ctx.Value(constants.ContainerKey).(*Container)
	if !ok || container == nil {
		return nil, fmt.Errorf("composition: container not found in context")
	}
	return container, nil
}
