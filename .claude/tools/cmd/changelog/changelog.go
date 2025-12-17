package changelog

import "github.com/spf13/cobra"

var ChangelogCmd = &cobra.Command{
	Use:   "changelog",
	Short: "CHANGELOG.md management",
	Long:  `CHANGELOG.md management utilities.`,
}

func init() {
	ChangelogCmd.AddCommand(checkCmd)
	ChangelogCmd.AddCommand(addCmd)
	ChangelogCmd.AddCommand(listCmd)
	ChangelogCmd.AddCommand(validateCmd)
}
