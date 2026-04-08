// Package server provides this package.
package server

import (
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bootstrap"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/server"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type DefaultOptions struct {
	Logger        *logrus.Logger
	Configuration *configuration.Configuration
	Application   application.Application
	Pool          *pgxpool.Pool
}

func Default(options *DefaultOptions) (*server.HTTPServer, error) {
	return server.New(&bootstrap.Runtime{
		Config: options.Configuration,
		Logger: options.Logger,
		Pool:   options.Pool,
		App:    options.Application,
	})
}
