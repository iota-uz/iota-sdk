package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	bichatpersistence "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	bichatservices "github.com/iota-uz/iota-sdk/modules/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/commands/common"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/spf13/cobra"
)

type knowledgeBootstrapOptions struct {
	KnowledgeDir string
	TenantID     string
	IndexPath    string
	MetadataDir  string
	Rebuild      bool
}

func runKnowledgeBootstrap(cmd *cobra.Command, opts knowledgeBootstrapOptions) error {
	conf := configuration.Use()

	knowledgeDir := strings.TrimSpace(opts.KnowledgeDir)
	if knowledgeDir == "" {
		knowledgeDir = strings.TrimSpace(conf.BiChatKnowledgeDir)
	}
	if knowledgeDir == "" {
		return fmt.Errorf("knowledge directory is required (--dir or BICHAT_KNOWLEDGE_DIR)")
	}

	tenantIDRaw := strings.TrimSpace(opts.TenantID)
	if tenantIDRaw == "" {
		return fmt.Errorf("tenant ID is required (--tenant-id)")
	}
	tenantID, err := uuid.Parse(tenantIDRaw)
	if err != nil {
		return fmt.Errorf("invalid tenant UUID: %w", err)
	}

	metadataDir := strings.TrimSpace(opts.MetadataDir)
	if metadataDir == "" {
		metadataDir = strings.TrimSpace(conf.BiChatSchemaMetadataDir)
	}
	if metadataDir == "" {
		metadataDir = filepath.Join(knowledgeDir, "tables")
	}

	indexPath := strings.TrimSpace(opts.IndexPath)
	if indexPath == "" {
		indexPath = strings.TrimSpace(conf.BiChatKBIndexPath)
	}

	pool, err := common.GetDefaultDatabasePool()
	if err != nil {
		return err
	}
	defer pool.Close()

	validatedStore := bichatpersistence.NewValidatedQueryRepository(pool)

	var (
		kbIndexer kb.KBIndexer
	)
	if indexPath != "" {
		if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
			return fmt.Errorf("failed to create KB index directory: %w", err)
		}
		indexer, _, err := kb.NewBleveIndex(indexPath)
		if err != nil {
			return fmt.Errorf("failed to initialize KB index: %w", err)
		}
		kbIndexer = indexer
		defer func() {
			_ = kbIndexer.Close()
		}()
	}

	bootstrap := bichatservices.NewKnowledgeBootstrapService(bichatservices.KnowledgeBootstrapConfig{
		ValidatedQueryStore: validatedStore,
		KBIndexer:           kbIndexer,
		MetadataOutputDir:   metadataDir,
	})

	result, err := bootstrap.Load(cmd.Context(), bichatservices.KnowledgeBootstrapRequest{
		TenantID:     tenantID,
		KnowledgeDir: knowledgeDir,
		Rebuild:      opts.Rebuild,
	})
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Knowledge %s complete\n", map[bool]string{true: "rebuild", false: "load"}[opts.Rebuild])
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  tables: %d\n", result.TableFilesLoaded)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  business: %d\n", result.BusinessFilesLoaded)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  query patterns: %d\n", result.QueryPatternsLoaded)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  validated queries saved: %d\n", result.ValidatedQueriesSaved)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  metadata files: %d\n", result.MetadataFilesGenerated)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  indexed docs: %d\n", result.KnowledgeDocsIndexed)

	return nil
}
