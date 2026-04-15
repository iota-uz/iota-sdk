// Package commands provides this package.
package commands

import (
	"github.com/spf13/cobra"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/bichatconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
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
			legacyConf := configuration.Use()
			bichatCfg := bichatconfig.FromLegacy(legacyConf)
			dbCfg := dbconfig.FromLegacy(legacyConf)
			return runKnowledgeBootstrap(cmd, knowledgeBootstrapOptions{
				KnowledgeDir: knowledgeDir,
				TenantID:     tenantID,
				IndexPath:    indexPath,
				MetadataDir:  metadataDir,
				Rebuild:      rebuild,
				BichatCfg:    &bichatCfg,
				DBCfg:        &dbCfg,
			})
		},
	}

	cmd.Flags().StringVar(&knowledgeDir, "dir", "", "Knowledge root directory containing tables/, queries/, business/")
	cmd.Flags().StringVar(&tenantID, "tenant-id", "", "Tenant UUID for validated query library writes")
	cmd.Flags().StringVar(&indexPath, "index-path", "", "Bleve index path for KB indexing")
	cmd.Flags().StringVar(&metadataDir, "metadata-dir", "", "Output directory for normalized table metadata JSON")

	return cmd
}
