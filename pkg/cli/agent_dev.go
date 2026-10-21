//go:build dev

package cli

import (
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/agentsignin"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/env"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/spf13/cobra"
)

func addAgentCommand(root *cobra.Command) {
	root.AddCommand(agentsignin.NewCommand(func() (agentsignin.Options, error) {
		src, err := config.Build(env.New(".env", ".env.local"))
		if err != nil {
			return agentsignin.Options{}, err
		}
		reg := config.NewRegistry(src)
		cfg, err := config.Register[agentsignin.Config](reg)
		if err != nil {
			return agentsignin.Options{}, err
		}
		app, err := config.Register[appconfig.Config](reg)
		if err != nil {
			return agentsignin.Options{}, err
		}
		tenant, err := uuid.Parse(cfg.TenantID)
		if err != nil {
			return agentsignin.Options{}, err
		}
		return agentsignin.Options{Enabled: cfg.Enabled, Environment: app.Environment, Origin: cfg.Origin, TenantID: tenant, Directory: cfg.Directory}, nil
	}))
}
