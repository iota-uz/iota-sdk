// Package commands provides this package.
package commands

import (
	"github.com/spf13/cobra"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/superadmin"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
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
			legacyConf := configuration.Use()
			cfg := dbconfig.FromLegacy(legacyConf)
			logger := legacyConf.Logger()
			allComponents := append(modules.Components(), superadmin.NewComponent(&superadmin.ModuleOptions{}))
			return CheckTrKeys(&cfg, logger, legacyConf, nil, allComponents...)
		},
	}
}
