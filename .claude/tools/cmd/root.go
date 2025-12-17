package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/iota-uz/iota-sdk/sdk-tools/cmd/auth"
	"github.com/iota-uz/iota-sdk/sdk-tools/cmd/changelog"
	"github.com/iota-uz/iota-sdk/sdk-tools/cmd/ci"
	"github.com/iota-uz/iota-sdk/sdk-tools/cmd/git"
	"github.com/iota-uz/iota-sdk/sdk-tools/cmd/gql"
	"github.com/iota-uz/iota-sdk/sdk-tools/cmd/pr"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "sdk-tools",
	Short: "Claude Code developer tools for IOTA SDK platform",
	Long: `sdk-tools is a comprehensive CLI toolkit designed to streamline development workflows
for the IOTA SDK platform when working with Claude Code.

This tool provides utilities for:
  - Authentication (login, session management for API testing)
  - Git workflow automation (branch naming, commit validation, etc.)
  - Pull request management (creation, review helpers, etc.)
  - CI/CD integration (workflow triggers, status checks, etc.)
  - CHANGELOG.md maintenance (automated generation and updates)

Each command is designed to integrate seamlessly with Claude Code's capabilities
and enforce best practices across the IOTA SDK codebase.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.sdk-tools.yaml)")

	// Add subcommands
	rootCmd.AddCommand(auth.AuthCmd)
	rootCmd.AddCommand(git.GitCmd)
	rootCmd.AddCommand(gql.GQLCmd)
	rootCmd.AddCommand(pr.PRCmd)
	rootCmd.AddCommand(ci.CICmd)
	rootCmd.AddCommand(changelog.ChangelogCmd)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// Search config in home directory with name ".sdk-tools" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".sdk-tools")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
