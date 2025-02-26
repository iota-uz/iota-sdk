package collector

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/auxten/postgresql-parser/pkg/sql/sem/tree"
	"github.com/iota-uz/iota-sdk/pkg/schema/common"
	"github.com/sirupsen/logrus"
)

type Collector struct {
	loader  *FileLoader
	dialect string
	logger  *logrus.Logger
	baseDir string
}

type Config struct {
	MigrationsPath string
	SQLDialect     string
	Logger         *logrus.Logger
	LogLevel       logrus.Level
	EmbedFSs       []*embed.FS
}

func New(cfg Config) *Collector {
	logger := cfg.Logger
	if logger == nil {
		logger = logrus.New()
		if cfg.LogLevel == 0 {
			cfg.LogLevel = logrus.InfoLevel
		}
		logger.SetLevel(cfg.LogLevel)
	}

	fileLoader := NewFileLoader(LoaderConfig{
		BaseDir:    cfg.MigrationsPath,
		Logger:     logger,
		EmbedFSs:   cfg.EmbedFSs,
	})

	return &Collector{
		loader:  fileLoader,
		dialect: cfg.SQLDialect,
		logger:  logger,
		baseDir: cfg.MigrationsPath,
	}
}

func (c *Collector) CollectMigrations(ctx context.Context) (*common.ChangeSet, error) {
	c.logger.Info("Starting migration collection")

	oldTree, err := c.loader.LoadExistingSchema(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing schema: %w", err)
	}

	newTree, err := c.loader.LoadModuleSchema(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load module schema: %w", err)
	}

	// Use the adapter to convert and compare schemas
	changes, err := CollectSchemaChanges(oldTree, newTree)
	if err != nil {
		c.logger.WithError(err).Error("Schema comparison failed")
		return nil, err
	}

	c.logger.Infof("Found %d changes", len(changes.Changes))

	return changes, nil
}

func (c *Collector) StoreMigrations(changes *common.ChangeSet) error {
	if changes == nil || len(changes.Changes) == 0 {
		c.logger.Info("No changes to store")
		return nil
	}

	c.logger.Info("Storing migrations")

	// Generate timestamp for the filename
	timestamp := fmt.Sprintf("%d", changes.Timestamp)
	if changes.Timestamp == 0 {
		// If timestamp is not set, use current Unix time
		timestamp = fmt.Sprintf("%d", time.Now().Unix())
	}

	// Create filename with timestamp
	filename := fmt.Sprintf("changes-%s.sql", timestamp)
	filepath := path.Join(c.baseDir, filename)

	c.logger.Infof("Creating migration file: %s", filepath)

	// Build SQL content
	var sqlContent strings.Builder
	sqlContent.WriteString("-- Generated migration\n\n")

	// Process each change and convert to SQL
	for i, change := range changes.Changes {
		switch node := change.(type) {
		case *tree.CreateTable:
			sqlContent.WriteString(fmt.Sprintf("-- Change CREATE_TABLE: %s\n", node.Table.TableName))
			sqlContent.WriteString(node.String())
			sqlContent.WriteString(";;\n\n")

		case *tree.AlterTableAddColumn:
			sqlContent.WriteString(fmt.Sprintf("-- Change ADD_COLUMN: %s\n", node.ColumnDef.Name))
			sqlContent.WriteString(node.String())
			sqlContent.WriteString(";;\n\n")

		case *tree.AlterTableAlterColumnType:
			sqlContent.WriteString(fmt.Sprintf("-- Change ALTER_COLUMN_TYPE: %s\n", node.Column))
			sqlContent.WriteString(node.String())
			sqlContent.WriteString(";;\n\n")

		case *tree.CreateIndex:
			sqlContent.WriteString(fmt.Sprintf("-- Change CREATE_INDEX: %s\n", node.Name))
			sqlContent.WriteString(node.String())
			sqlContent.WriteString(";;\n\n")

		default:
			c.logger.Warnf("Unknown change type at index %d: %T", i, change)
			sqlContent.WriteString(fmt.Sprintf("-- Unknown change type: %T\n", change))
			// Try to use String() method if available via reflection
			if stringer, ok := change.(fmt.Stringer); ok {
				sqlContent.WriteString(stringer.String())
				sqlContent.WriteString(";;\n\n")
			}
		}
	}

	// Write the file
	err := os.WriteFile(filepath, []byte(sqlContent.String()), 0644)
	if err != nil {
		return fmt.Errorf("failed to write migration file: %w", err)
	}

	c.logger.Infof("Successfully stored migrations to %s", filepath)
	return nil
}
