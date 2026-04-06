package seed

import (
	"github.com/iota-uz/iota-sdk/modules/website/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

func RegisterProviders(deps *application.SeedDeps) {
	deps.RegisterValues(persistence.NewAIChatConfigRepository())
}
