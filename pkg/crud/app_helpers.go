// Package crud provides this package.
package crud

import (
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

func GetBuilder[TEntity any](app application.Application) Builder[TEntity] {
	builder, err := composition.ResolveForApp[Builder[TEntity]](app)
	if err != nil {
		panic(err)
	}
	return builder
}
