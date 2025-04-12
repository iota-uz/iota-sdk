package collector

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/schema/common"
	"github.com/iota-uz/psql-parser/sql/sem/tree"
	"github.com/sirupsen/logrus"
)

type Collector struct {
	loader  *FileLoader
	logger  *logrus.Logger
	baseDir string
}

type Config struct {
	MigrationsPath string
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
		BaseDir:  cfg.MigrationsPath,
		Logger:   logger,
		EmbedFSs: cfg.EmbedFSs,
	})

	return &Collector{
		loader:  fileLoader,
		logger:  logger,
		baseDir: cfg.MigrationsPath,
	}
}

func (c *Collector) CollectMigrations(ctx context.Context) (*common.ChangeSet, *common.ChangeSet, error) {
	c.logger.Info("Starting migration collection")

	oldTree, err := c.loader.LoadExistingSchema(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load existing schema: %w", err)
	}

	newTree, err := c.loader.LoadModuleSchema(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load module schema: %w", err)
	}

	// Use the adapter to convert and compare schemas
	upChanges, downChanges, err := CollectSchemaChanges(oldTree, newTree)
	if err != nil {
		c.logger.WithError(err).Error("Schema comparison failed")
		return nil, nil, err
	}

	c.logger.Infof("Found %d up changes and %d down changes",
		len(upChanges.Changes), len(downChanges.Changes))

	return upChanges, downChanges, nil
}

func (c *Collector) StoreMigrations(upChanges, downChanges *common.ChangeSet) error {
	if (upChanges == nil || len(upChanges.Changes) == 0) && (downChanges == nil || len(downChanges.Changes) == 0) {
		c.logger.Info("No changes to store")
		return nil
	}

	c.logger.Info("Storing migrations")

	// Generate timestamp for the filename
	timestamp := fmt.Sprintf("%d", upChanges.Timestamp)
	if upChanges.Timestamp == 0 {
		// If timestamp is not set, use current Unix time
		timestamp = fmt.Sprintf("%d", time.Now().Unix())
	}

	// Create filename with timestamp
	filename := fmt.Sprintf("changes-%s.sql", timestamp)
	filepath := path.Join(c.baseDir, filename)

	c.logger.Infof("Creating migration file: %s", filepath)

	// string builder
	buffer := &bytes.Buffer{}
	pPrinter := tree.PrettyCfg{
		LineWidth: 120,
		Simplify:  false,
		TabWidth:  4,
		UseTabs:   true,
		Align:     tree.PrettyAlignOnly,
	}

	// Up migrations
	buffer.WriteString("-- +migrate Up\n\n")

	if upChanges != nil && len(upChanges.Changes) > 0 {
		// Process each up change and convert to SQL
		for i, change := range upChanges.Changes {
			switch node := change.(type) {
			case *tree.CreateTable:
				buffer.WriteString(fmt.Sprintf("-- Change CREATE_TABLE: %s\n", node.Table.TableName))
				buffer.WriteString(pPrinter.Pretty(node))
				buffer.WriteString(";\n\n")

			case *tree.AlterTable:
				// Handle each command in the AlterTable
				for _, cmd := range node.Cmds {
					switch altCmd := cmd.(type) {
					case *tree.AlterTableAddColumn:
						buffer.WriteString(fmt.Sprintf("-- Change ADD_COLUMN: %s\n", altCmd.ColumnDef.Name))
						buffer.WriteString(pPrinter.Pretty(node))
						buffer.WriteString(";\n\n")
					case *tree.AlterTableAlterColumnType:
						buffer.WriteString(fmt.Sprintf("-- Change ALTER_COLUMN_TYPE: %s\n", altCmd.Column))
						buffer.WriteString(pPrinter.Pretty(node))
						buffer.WriteString(";\n\n")
					default:
						buffer.WriteString(pPrinter.Pretty(node))
						buffer.WriteString(";\n\n")
					}
				}
			case *tree.CreateIndex:
				buffer.WriteString(fmt.Sprintf("-- Change CREATE_INDEX: %s\n", node.Name))
				buffer.WriteString(pPrinter.Pretty(node))
				buffer.WriteString(";\n\n")
			default:
				c.logger.Warnf("Unknown up change type at index %d: %T", i, change)
				buffer.WriteString(fmt.Sprintf("-- Unknown change type: %T\n", change))
				// Try to use String() method if available via reflection
				if stringer, ok := change.(fmt.Stringer); ok {
					buffer.WriteString(stringer.String())
					buffer.WriteString(";\n\n")
				}
			}
		}
	}

	// Down migrations
	buffer.WriteString("\n-- +migrate Down\n\n")

	if downChanges != nil && len(downChanges.Changes) > 0 {
		// Process each down change and convert to SQL
		for i, change := range downChanges.Changes {
			switch node := change.(type) {
			case *tree.DropTable:
				buffer.WriteString(fmt.Sprintf("-- Undo CREATE_TABLE: %s\n", node.Names[0].TableName))
				buffer.WriteString(node.String())
				buffer.WriteString(";\n\n")

			case *tree.AlterTable:
				// Handle each command in the AlterTable
				for _, cmd := range node.Cmds {
					switch altCmd := cmd.(type) {
					case *tree.AlterTableDropColumn:
						buffer.WriteString(fmt.Sprintf("-- Undo ADD_COLUMN: %s\n", altCmd.Column))
						buffer.WriteString(pPrinter.Pretty(node))
						buffer.WriteString(";\n\n")
					case *tree.AlterTableAlterColumnType:
						buffer.WriteString(fmt.Sprintf("-- Undo ALTER_COLUMN_TYPE: %s\n", altCmd.Column))
						buffer.WriteString(pPrinter.Pretty(node))
						buffer.WriteString(";\n\n")
					default:
						buffer.WriteString(pPrinter.Pretty(node))
						buffer.WriteString(";\n\n")
					}
				}

			case *tree.DropIndex:
				buffer.WriteString(fmt.Sprintf("-- Undo CREATE_INDEX: %s\n", node.IndexList[0]))
				buffer.WriteString(node.String())
				buffer.WriteString(";\n\n")

			default:
				c.logger.Warnf("Unknown down change type at index %d: %T", i, change)
				buffer.WriteString(fmt.Sprintf("-- Unknown down change type: %T\n", change))
				// Try to use String() method if available via reflection
				if stringer, ok := change.(fmt.Stringer); ok {
					buffer.WriteString(stringer.String())
					buffer.WriteString(";\n\n")
				}
			}
		}
	}

	// Write the file
	err := os.WriteFile(filepath, buffer.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("failed to write migration file: %w", err)
	}

	c.logger.Infof("Successfully stored migrations to %s", filepath)
	return nil
}
