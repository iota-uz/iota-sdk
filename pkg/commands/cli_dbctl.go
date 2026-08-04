// Package commands provides this package.
package commands

import (
	dbctlcli "github.com/iota-uz/iota-sdk/pkg/dbctl/cli"
	"github.com/spf13/cobra"
)

func NewDBCtlCommand() *cobra.Command {
	return dbctlcli.NewCommand()
}
