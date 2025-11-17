package gql

import (
	"os"

	"github.com/iota-uz/iota-sdk/sdk-tools/internal/output"
	"github.com/spf13/cobra"
)

var (
	query     string
	queryFile string
	token     string
	variables string
	jsonFlag  bool
)

var GQLCmd = &cobra.Command{
	Use:   "gql",
	Short: "GraphQL query utility",
	Long:  `Execute GraphQL queries against the API.`,
	RunE:  runGQL,
}

func init() {
	GQLCmd.Flags().StringVar(&query, "query", "", "GraphQL query string")
	GQLCmd.Flags().StringVar(&queryFile, "file", "", "GraphQL query file")
	GQLCmd.Flags().StringVar(&token, "token", "", "API token")
	GQLCmd.Flags().StringVar(&variables, "variables", "", "Query variables as JSON")
	GQLCmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON")
}

func runGQL(cmd *cobra.Command, args []string) error {
	formatter := output.New(os.Stdout, jsonFlag)

	// TODO: Implement GraphQL query execution

	if formatter.IsJSON() {
		return formatter.PrintJSON(map[string]interface{}{
			"message": "GraphQL support not yet implemented",
		})
	}

	return formatter.PrintTextLn("GraphQL support not yet implemented")
}
