package composition

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

var appContainers sync.Map

func Attach(app application.Application, container *Container) {
	if app == nil || container == nil {
		return
	}
	appContainers.Store(app, container)
}

func Detach(app application.Application) {
	if app == nil {
		return
	}
	appContainers.Delete(app)
}

func ForApp(app application.Application) (*Container, bool) {
	if app == nil {
		return nil, false
	}
	container, ok := appContainers.Load(app)
	if !ok {
		return nil, false
	}
	typed, ok := container.(*Container)
	return typed, ok && typed != nil
}

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

func ResolveForApp[T any](app application.Application) (T, error) {
	container, ok := ForApp(app)
	if !ok {
		var zero T
		return zero, fmt.Errorf("composition: container not attached to application")
	}
	return Resolve[T](container)
}

func ResolveOptionalForApp[T any](app application.Application) (T, bool, error) {
	container, ok := ForApp(app)
	if !ok {
		var zero T
		return zero, false, nil
	}
	return UseOptional[T]().Resolve(container)
}

func MustResolveForApp[T any](app application.Application) T {
	value, err := ResolveForApp[T](app)
	if err != nil {
		panic(err)
	}
	return value
}

func ResolveAnyForApp(app application.Application, exemplar any) (any, error) {
	container, ok := ForApp(app)
	if !ok {
		return nil, fmt.Errorf("composition: container not attached to application")
	}
	return resolveAnyByExample(container, exemplar)
}

func resolveAnyByExample(container *Container, exemplar any) (any, error) {
	if container == nil {
		return nil, fmt.Errorf("composition: container is nil")
	}
	if exemplar == nil {
		return nil, fmt.Errorf("composition: exemplar is nil")
	}

	candidates := make([]reflect.Type, 0, 2)
	exemplarType := reflect.TypeOf(exemplar)
	candidates = append(candidates, exemplarType)
	if exemplarType.Kind() != reflect.Ptr {
		candidates = append(candidates, reflect.PointerTo(exemplarType))
	}

	for _, candidate := range candidates {
		value, err := container.resolveAny(keyFor(candidate, ""))
		if err == nil {
			return value, nil
		}
		if !IsNotProvided(err) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("composition: %s not provided", exemplarType)
}
