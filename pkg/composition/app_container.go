package composition

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

// RequireApplication returns the application.Application handle associated
// with the container. Components that need the app (to pass into controller
// constructors, for example) call this from inside a ContributeControllers
// closure.
func RequireApplication(container *Container) (application.Application, error) {
	if container == nil || container.context.app == nil {
		return nil, fmt.Errorf("composition: application is nil")
	}
	return container.context.app, nil
}

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
