package collector

import (
	"context"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/schema/common"
	"github.com/iota-uz/iota-sdk/pkg/schema/diff"
	"github.com/sirupsen/logrus"
)

type Collector struct {
	loader  *FileLoader
	parser  Parser
	dialect string
	logger  *logrus.Logger
	baseDir string
}

type Config struct {
	ModulesPath    string
	MigrationsPath string
	SQLDialect     string
	Logger         *logrus.Logger
	LogLevel       logrus.Level
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

	sqlParser := NewPostgresParser(logger)

	fileLoader := NewFileLoader(LoaderConfig{
		BaseDir:    cfg.MigrationsPath,
		ModulesDir: cfg.ModulesPath,
		Parser:     sqlParser,
		Logger:     logger,
	})

	return &Collector{
		loader:  fileLoader,
		parser:  sqlParser,
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
	c.logSchemaDetails("Existing", oldTree)

	newTree, err := c.loader.LoadModuleSchema(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load module schema: %w", err)
	}
	c.logSchemaDetails("Module", newTree)

	changes, err := c.compareSchemas(oldTree, newTree)
	if err != nil {
		return nil, fmt.Errorf("failed to compare schemas: %w", err)
	}

	c.enrichChanges(changes, newTree)
	return changes, nil
}

func (c *Collector) StoreMigrations(changes *common.ChangeSet) error {
	if changes == nil || len(changes.Changes) == 0 {
		c.logger.Info("No changes to store")
		return nil
	}

	c.logChangeDetails(changes)

	generator, err := c.createMigrationGenerator()
	if err != nil {
		return fmt.Errorf("failed to create migration generator: %w", err)
	}

	if err := generator.Generate(changes); err != nil {
		c.logger.WithError(err).Error("Failed to generate migrations")
		return fmt.Errorf("failed to generate migrations: %w", err)
	}

	c.logger.Info("Successfully stored migrations")
	return nil
}

// Private helper methods

func (c *Collector) logSchemaDetails(schemaType string, tree *SchemaTree) {
	c.logger.Infof("Loaded %s schema with %d tables", schemaType, len(tree.Root.Children))

	if c.logger.IsLevelEnabled(logrus.DebugLevel) {
		for _, node := range tree.Root.Children {
			if node.Type == NodeTable {
				c.logger.Debugf("%s schema table: %s with %d columns",
					schemaType, node.Name, len(node.Children))

				for _, col := range node.Children {
					if col.Type == NodeColumn {
						c.logger.Debugf("  Column: %s, Type: %s, Constraints: %s",
							col.Name,
							col.Metadata["type"],
							col.Metadata["constraints"])
					}
				}
			}
		}
	}
}

func (c *Collector) compareSchemas(oldTree, newTree *SchemaTree) (*common.ChangeSet, error) {
	c.logger.Info("Comparing schemas")

	// Use the adapter to convert and compare schemas
	changes, err := CollectSchemaChanges(oldTree, newTree)
	if err != nil {
		c.logger.WithError(err).Error("Schema comparison failed")
		return nil, err
	}

	c.logger.Infof("Found %d changes", len(changes.Changes))
	return changes, nil
}

func (c *Collector) enrichChanges(changes *common.ChangeSet, newTree *SchemaTree) {
	for _, change := range changes.Changes {
		if change.Type == common.CreateTable {
			// We're using the postgresql-parser types directly now
			// so object enrichment happens in the adapter.ToSchema() method
			c.logger.Debugf("CREATE TABLE change for table: %s", change.ObjectName)
		}
	}
}


func (c *Collector) findTableInSchema(tableName string, schema *SchemaTree) *Node {
	for _, node := range schema.Root.Children {
		if node.Type == NodeTable && strings.EqualFold(node.Name, tableName) {
			return c.enrichTableNode(node)
		}
	}
	return nil
}

func (c *Collector) enrichTableNode(node *Node) *Node {
	if node == nil || node.Type != NodeTable {
		return node
	}

	enriched := &Node{
		Type:     node.Type,
		Name:     node.Name,
		Children: make([]*Node, len(node.Children)),
		Metadata: make(map[string]interface{}),
	}

	// Copy metadata
	for k, v := range node.Metadata {
		enriched.Metadata[k] = v
	}

	// Copy and enrich children
	for i, child := range node.Children {
		enriched.Children[i] = c.enrichChildNode(child)
	}

	return enriched
}

func (c *Collector) enrichChildNode(child *Node) *Node {
	enriched := &Node{
		Type:     child.Type,
		Name:     child.Name,
		Children: make([]*Node, len(child.Children)),
		Metadata: make(map[string]interface{}),
	}

	// Copy child metadata
	for k, v := range child.Metadata {
		enriched.Metadata[k] = v
	}

	// Ensure column definitions are complete
	if child.Type == NodeColumn {
		if _, ok := child.Metadata["definition"]; !ok {
			enriched.Metadata["definition"] = c.buildColumnDefinition(child)
		}
	}

	return enriched
}

func (c *Collector) buildColumnDefinition(column *Node) string {
	typeInfo := column.Metadata["type"].(string)
	if rawType, ok := column.Metadata["rawType"].(string); ok {
		typeInfo = rawType
	}
	return fmt.Sprintf("%s %s", column.Name, typeInfo)
}

func (c *Collector) createMigrationGenerator() (*diff.Generator, error) {
	return diff.NewGenerator(diff.GeneratorOptions{
		Dialect:        c.dialect,
		OutputDir:      c.baseDir,
		FileNameFormat: "changes-%d.sql",
		IncludeDown:    true,
		Logger:         c.logger,
	})
}

func (c *Collector) logChangeDetails(changes *common.ChangeSet) {
	for _, change := range changes.Changes {
		logEntry := c.logger.WithFields(logrus.Fields{
			"type":   change.Type,
			"table":  change.ObjectName,
			"parent": change.ParentName,
		})

		if change.Object != nil {
			logEntry = logEntry.WithField("metadata", change.Object)
		}

		logEntry.Debug("Change details")
	}
}
