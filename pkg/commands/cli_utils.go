// Package commands provides this package.
package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/superadmin"
	"github.com/iota-uz/iota-sdk/pkg/config"
	envprov "github.com/iota-uz/iota-sdk/pkg/config/providers/env"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
)

// NewUtilityCommands creates utility commands.
func NewUtilityCommands() []*cobra.Command {
	return []*cobra.Command{
		newCheckTrKeysCmd(),
	}
}

func newCheckTrKeysCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check_tr_keys",
		Short: "Check translation key consistency across all locales",
		Long:  `Validates that all translation keys are present across all configured locales and reports any missing translations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := config.Build(envprov.New(".env", ".env.local"))
			if err != nil {
				return fmt.Errorf("failed to build config source: %w", err)
			}
			reg := config.NewRegistry(src)
			cfg, err := config.Register[dbconfig.Config](reg, "db")
			if err != nil {
				return fmt.Errorf("failed to load dbconfig: %w", err)
			}
			allComponents := append(modules.Components(), superadmin.NewComponent(&superadmin.ModuleOptions{}))
			return CheckTrKeys(cfg, src, nil, nil, allComponents...)
		},
	}
}
