package crud

import "github.com/iota-uz/iota-sdk/pkg/application"

func GetBuilder[TEntity any](app application.Application) Builder[TEntity] {
	return app.Service(builder[TEntity]{}).(Builder[TEntity])
}
