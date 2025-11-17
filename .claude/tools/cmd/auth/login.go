package auth

import (
	"os"

	"github.com/iota-uz/iota-sdk/sdk-tools/internal/output"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to get API token",
	Long:  `Login using preset credentials to get an API token.`,
	RunE:  runLogin,
}

var (
	loginPortFlag int
	loginJSONFlag bool
)

func init() {
	loginCmd.Flags().IntVar(&loginPortFlag, "port", 3000, "Local port for login server")
	loginCmd.Flags().BoolVar(&loginJSONFlag, "json", false, "Output as JSON")
}

func runLogin(cmd *cobra.Command, args []string) error {
	formatter := output.New(os.Stdout, loginJSONFlag)

	// TODO: Implement OAuth login flow
	// Preset user: test@gmail.com / TestPass123!

	if formatter.IsJSON() {
		return formatter.PrintJSON(map[string]interface{}{
			"token": "example_token_placeholder",
		})
	}

	return formatter.PrintTextLn("Login successful")
}
