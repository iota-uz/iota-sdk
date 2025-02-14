package website

import (
	"github.com/iota-uz/iota-sdk/modules/website/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

//go:generate go run github.com/99designs/gqlgen generate

////go:embed presentation/locales/*.json
// var localeFiles embed.FS

////go:embed infrastructure/persistence/schema/warehouse-schema.sql
//var migrationFiles embed.FS

func NewModule() application.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Register(app application.Application) error {
	app.RegisterControllers(
		controllers.NewAIChatController(app),
	)
	return nil
}

func (m *Module) Name() string {
	return "website"
}
