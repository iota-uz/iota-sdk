//go:build dev

package agentsignin

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// NewCommand loads host configuration only when sign-in is invoked.
func NewCommand(load func() (Options, error)) *cobra.Command {
	var user, next, output string
	root := &cobra.Command{Use: "agent", Short: "Local-development agent tools"}
	cmd := &cobra.Command{Use: "sign-in", Short: "Print a single-use browser sign-in URL", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if output != "console" && output != "plain" && output != "json" {
				return fmt.Errorf("unsupported output format: %s", output)
			}
			opts, err := load()
			if err != nil {
				return err
			}
			link, err := Issue(opts, user, next)
			if err != nil {
				return err
			}
			if output == "json" {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"url": link})
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), link)
			return err
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "User ID or exact email within the configured tenant")
	cmd.Flags().StringVar(&next, "next", "/", "Internal page to open after sign-in")
	cmd.Flags().StringVar(&output, "output", "plain", "Output format: console, plain, json")
	_ = cmd.MarkFlagRequired("user")
	root.AddCommand(cmd)
	return root
}
