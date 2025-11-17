package auth

import "github.com/spf13/cobra"

var AuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication utilities",
	Long:  `Authentication utilities for API access and testing.`,
}

func init() {
	AuthCmd.AddCommand(loginCmd)
}
