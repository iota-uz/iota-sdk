// Package commands provides this package.
package commands

import (
	"github.com/spf13/cobra"

	"github.com/iota-uz/iota-sdk/pkg/config"
	envprov "github.com/iota-uz/iota-sdk/pkg/config/providers/env"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/bichatconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
)

// NewKnowledgeCommand creates the knowledge command group.
func NewKnowledgeCommand() *cobra.Command {
	knowledgeCmd := &cobra.Command{
		Use:   "knowledge",
		Short: "Manage BI knowledge artifacts",
		Long:  `Load or rebuild static BI knowledge artifacts (tables, queries, business) into configured stores.`,
	}

	knowledgeCmd.AddCommand(newKnowledgeLoadCmd(false))
	knowledgeCmd.AddCommand(newKnowledgeLoadCmd(true))

	return knowledgeCmd
}

func newKnowledgeLoadCmd(rebuild bool) *cobra.Command {
	use := "load"
	short := "Load knowledge artifacts with upsert semantics"
	if rebuild {
		use = "rebuild"
		short = "Rebuild knowledge artifacts from scratch"
	}

	var (
		knowledgeDir string
		tenantID     string
		indexPath    string
		metadataDir  string
	)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := config.Build(envprov.New(".env", ".env.local"))
			if err != nil {
				return err
			}
			reg := config.NewRegistry(src)
			bichatCfg, err := config.Register[bichatconfig.Config](reg, "bichat")
			if err != nil {
				return err
			}
			dbCfg, err := config.Register[dbconfig.Config](reg, "db")
			if err != nil {
				return err
			}
			return runKnowledgeBootstrap(cmd, knowledgeBootstrapOptions{
				KnowledgeDir: knowledgeDir,
				TenantID:     tenantID,
				IndexPath:    indexPath,
				MetadataDir:  metadataDir,
				Rebuild:      rebuild,
				BichatCfg:    bichatCfg,
				DBCfg:        dbCfg,
			})
		},
	}

	cmd.Flags().StringVar(&knowledgeDir, "dir", "", "Knowledge root directory containing tables/, queries/, business/")
	cmd.Flags().StringVar(&tenantID, "tenant-id", "", "Tenant UUID for validated query library writes")
	cmd.Flags().StringVar(&indexPath, "index-path", "", "Bleve index path for KB indexing")
	cmd.Flags().StringVar(&metadataDir, "metadata-dir", "", "Output directory for normalized table metadata JSON")

	return cmd
}
