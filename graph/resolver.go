package graph

import (
	coregraph "github.com/iota-uz/iota-sdk/modules/core/interfaces/graph"
	warehousegraph "github.com/iota-uz/iota-sdk/modules/warehouse/interfaces/graph"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	app               application.Application
	coreResolver      *coregraph.Resolver
	warehouseResolver *warehousegraph.Resolver
}

func NewResolver(app application.Application) *Resolver {
	coreResolver := coregraph.NewResolver(app)
	warehouseResolver := warehousegraph.NewResolver(app)
	return &Resolver{
		app:               app,
		coreResolver:      coreResolver,
		warehouseResolver: warehouseResolver,
	}
}
