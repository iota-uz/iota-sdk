// Package commands provides this package.
package commands

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/superadmin"
)

// NewUtilityCommands creates utility commands.
func NewUtilityCommands() []*cobra.Command {
	return []*cobra.Command{
		newCheckTrKeysCmd(),
	}
}

func newCheckTrKeysCmd() *cobra.Command {
	var rootPath string
	cmd := &cobra.Command{
		Use:   "check_tr_keys",
		Short: "Check translation key consistency across all locales",
		Long:  `Validates that all translation keys are present across all configured locales and reports any missing translations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if rootPath == "" {
				wd, err := os.Getwd()
				if err != nil {
					return err
				}
				rootPath = wd
			}
			allComponents := append(modules.Components(), superadmin.NewComponent(&superadmin.ModuleOptions{}))
			return CheckTrKeys(nil, rootPath, allComponents...)
		},
	}
	cmd.Flags().StringVar(&rootPath, "root", "", "Project root to scan for T()/TSafe() call sites (defaults to current working directory)")
	return cmd
}
