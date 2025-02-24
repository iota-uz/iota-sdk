package collector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/schema/ast"
	"github.com/iota-uz/iota-sdk/pkg/schema/dialect"
	"github.com/iota-uz/iota-sdk/pkg/schema/diff"
	"github.com/iota-uz/iota-sdk/pkg/schema/types"
	"github.com/sirupsen/logrus"
)

// Collector handles collecting and analyzing migrations from modules
type Collector struct {
	baseDir    string
	modulesDir string
	parser     *ast.Parser
	migrations map[string]*types.SchemaTree
	dialect    dialect.Dialect
	logger     *logrus.Logger
}

// Config holds collector configuration
type Config struct {
	ModulesPath    string
	MigrationsPath string
	SQLDialect     string
	Logger         *logrus.Logger
	LogLevel       logrus.Level
}

// New creates a new migration collector
func New(cfg Config) *Collector {
	d, ok := dialect.Get(cfg.SQLDialect)
	if !ok {
		d = dialect.NewPostgresDialect() // Default to PostgreSQL
	}

	logger := cfg.Logger
	if logger == nil {
		logger = logrus.New()
		// Default log level to INFO if not configured
		if cfg.LogLevel == 0 {
			cfg.LogLevel = logrus.InfoLevel
		}
		// logger.SetLevel(cfg.LogLevel)
	} else {
		logger.SetLevel(cfg.LogLevel)
	}

	return &Collector{
		baseDir:    cfg.MigrationsPath,
		modulesDir: cfg.ModulesPath,
		parser:     ast.NewParser(cfg.SQLDialect, ast.ParserOptions{StrictMode: true}),
		migrations: make(map[string]*types.SchemaTree),
		dialect:    d,
		logger:     logger,
	}
}

// CollectMigrations gathers all migrations from modules and analyzes changes
func (c *Collector) CollectMigrations(ctx context.Context) (*diff.ChangeSet, error) {
	c.logger.Info("Starting CollectMigrations")

	oldTree, err := c.loadExistingSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to load existing schema: %w", err)
	}
	c.logger.Infof("Loaded existing schema with %d tables", len(oldTree.Root.Children))
	for _, node := range oldTree.Root.Children {
		if node.Type == types.NodeTable {
			c.logger.Debugf("Existing schema table: %s with %d columns", node.Name, len(node.Children))
		}
	}

	newTree, err := c.loadModuleSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to load module schema: %w", err)
	}
	c.logger.Debugf("Loaded module schema with %d tables", len(newTree.Root.Children))
	for _, node := range newTree.Root.Children {
		if node.Type == types.NodeTable {
			c.logger.Debugf("Module schema table: %s with %d columns", node.Name, len(node.Children))
		}
	}

	c.logger.Info("Creating analyzer for schema comparison")
	analyzer := diff.NewAnalyzer(oldTree, newTree, diff.AnalyzerOptions{
		IgnoreCase:          true,
		IgnoreWhitespace:    true,
		DetectRenames:       true,
		ValidateConstraints: true,
	})

	changes, err := analyzer.Compare()
	if err != nil {
		c.logger.Errorf("Error during comparison: %v", err)
		return nil, err
	}

	// Ensure each CREATE TABLE change has complete column information
	for i, change := range changes.Changes {
		if change.Type == diff.CreateTable {
			if node := c.findTableInSchema(change.ObjectName, newTree); node != nil {
				// Replace the change object with complete node information
				changes.Changes[i].Object = node
			}
		}
	}

	return changes, nil
}

func (c *Collector) findTableInSchema(tableName string, schema *types.SchemaTree) *types.Node {
	for _, node := range schema.Root.Children {
		if node.Type == types.NodeTable && strings.EqualFold(node.Name, tableName) {
			return c.enrichTableNode(node)
		}
	}
	return nil
}

func (c *Collector) enrichTableNode(node *types.Node) *types.Node {
	if node == nil || node.Type != types.NodeTable {
		return node
	}

	// Create a new node to avoid modifying the original
	enriched := &types.Node{
		Type:     node.Type,
		Name:     node.Name,
		Children: make([]*types.Node, len(node.Children)),
		Metadata: make(map[string]interface{}),
	}

	// Copy metadata
	for k, v := range node.Metadata {
		enriched.Metadata[k] = v
	}

	// Copy and enrich children
	for i, child := range node.Children {
		enriched.Children[i] = &types.Node{
			Type:     child.Type,
			Name:     child.Name,
			Children: make([]*types.Node, len(child.Children)),
			Metadata: make(map[string]interface{}),
		}

		// Copy child metadata
		for k, v := range child.Metadata {
			enriched.Children[i].Metadata[k] = v
		}

		// Ensure column definitions are complete
		if child.Type == types.NodeColumn {
			if _, ok := child.Metadata["definition"]; !ok {
				// Build complete definition if missing
				typeInfo := child.Metadata["type"].(string)
				rawType := child.Metadata["rawType"]
				if rawType != nil {
					typeInfo = rawType.(string)
				}
				enriched.Children[i].Metadata["definition"] = fmt.Sprintf("%s %s", child.Name, typeInfo)
			}
		}
	}

	return enriched
}

func (c *Collector) loadExistingSchema() (*types.SchemaTree, error) {
	tree := ast.NewSchemaTree()
	c.logger.Infof("Loading existing schema files from: %s", c.baseDir)

	// Read migration files
	files, err := os.ReadDir(c.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			c.logger.Infof("No existing migrations directory found at: %s", c.baseDir)
			return tree, nil
		}
		return nil, err
	}

	// Collect and sort migration files
	var migrationFiles []string
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") || strings.HasSuffix(file.Name(), ".down.sql") {
			continue
		}
		migrationFiles = append(migrationFiles, file.Name())
	}

	// Sort files by timestamp in filename
	sort.Slice(migrationFiles, func(i, j int) bool {
		// Extract timestamps from filenames (format: changes-TIMESTAMP.sql)
		tsI := strings.TrimSuffix(strings.TrimPrefix(migrationFiles[i], "changes-"), ".sql")
		tsJ := strings.TrimSuffix(strings.TrimPrefix(migrationFiles[j], "changes-"), ".sql")
		numI, errI := strconv.ParseInt(tsI, 10, 64)
		numJ, errJ := strconv.ParseInt(tsJ, 10, 64)
		if errI != nil || errJ != nil {
			return migrationFiles[i] < migrationFiles[j] // Fallback to string comparison
		}
		return numI < numJ
	})

	// Track the latest state of each column and index with its type and timestamp
	type ColumnState struct {
		Node      *types.Node
		Timestamp int64
		Type      string
		LastFile  string
	}
	type IndexState struct {
		Node      *types.Node
		Timestamp int64
		LastFile  string
	}
	tableStates := make(map[string]map[string]*ColumnState) // table -> column -> state
	indexStates := make(map[string]*IndexState)             // index -> state

	// Process migrations in chronological order
	for _, fileName := range migrationFiles {
		c.logger.Infof("Processing migration file: %s", fileName)
		timestamp := strings.TrimSuffix(strings.TrimPrefix(fileName, "changes-"), ".sql")
		ts, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			c.logger.Warnf("Invalid timestamp in filename %s: %v", fileName, err)
			continue
		}

		path := filepath.Join(c.baseDir, fileName)
		content, err := os.ReadFile(path)
		if err != nil {
			c.logger.Warnf("Failed to read file %s: %v", path, err)
			continue
		}

		sql := string(content)
		parsed, err := c.parser.Parse(sql)
		if err != nil {
			c.logger.Warnf("Failed to parse file %s: %v", path, err)
			continue
		}

		// Handle ALTER TABLE statements specifically
		if strings.Contains(strings.ToUpper(sql), "ALTER TABLE") {
			// Parse each statement more carefully to handle semicolons in definitions
			statements := strings.Split(sql, ";")
			for _, stmt := range statements {
				stmt = strings.TrimSpace(stmt)
				if stmt == "" {
					continue
				}

				if strings.Contains(strings.ToUpper(stmt), "ALTER COLUMN") {
					parts := strings.Fields(stmt)
					if len(parts) >= 7 && strings.EqualFold(parts[0], "ALTER") && strings.EqualFold(parts[1], "TABLE") {
						tableName := strings.ToLower(parts[2])
						columnName := strings.ToLower(parts[5])

						// Find the TYPE keyword to properly extract the type definition
						typeIdx := -1
						for i, part := range parts {
							if strings.EqualFold(part, "TYPE") {
								typeIdx = i
								break
							}
						}

						if typeIdx > 0 && typeIdx < len(parts)-1 {
							// Extract everything after TYPE keyword until any trailing keywords
							typeEnd := len(parts)
							for i := typeIdx + 1; i < len(parts); i++ {
								upper := strings.ToUpper(parts[i])
								if upper == "SET" || upper == "DROP" || upper == "USING" {
									typeEnd = i
									break
								}
							}

							// Join the type parts together
							newType := strings.Join(parts[typeIdx+1:typeEnd], " ")
							newType = strings.TrimRight(newType, ";")

							if tableState, exists := tableStates[tableName]; exists {
								if currentState, exists := tableState[columnName]; exists {
									c.logger.Debugf("Updating column type from ALTER statement: %s.%s to %s",
										tableName, columnName, newType)

									// Update the column state with the new type
									currentState.Type = newType
									if currentState.Node.Metadata == nil {
										currentState.Node.Metadata = make(map[string]interface{})
									}
									currentState.Node.Metadata["type"] = newType
									currentState.Node.Metadata["fullType"] = newType
									// Update the full definition to match the new type
									currentState.Node.Metadata["definition"] = fmt.Sprintf("%s %s", columnName, newType)
									if constraints, ok := currentState.Node.Metadata["constraints"].(string); ok && constraints != "" {
										currentState.Node.Metadata["definition"] = fmt.Sprintf("%s %s %s",
											columnName, newType, strings.TrimSpace(constraints))
									}
									currentState.Timestamp = ts
									currentState.LastFile = fileName
								}
							}
						}
					}
				}
			}
		}

		// Update table and index states with changes from this migration
		for _, node := range parsed.Root.Children {
			switch node.Type {
			case types.NodeTable:
				tableName := strings.ToLower(node.Name)
				if _, exists := tableStates[tableName]; !exists {
					tableStates[tableName] = make(map[string]*ColumnState)
				}

				// Process each column
				for _, col := range node.Children {
					if col.Type == types.NodeColumn {
						colName := strings.ToLower(col.Name)
						currentState := tableStates[tableName][colName]

						// Get the new type information
						newType := ""
						if fullType, ok := col.Metadata["fullType"].(string); ok {
							newType = strings.ToLower(strings.TrimRight(fullType, ";"))
						} else if typeStr, ok := col.Metadata["type"].(string); ok {
							newType = strings.ToLower(strings.TrimRight(typeStr, ";"))
						}

						// Only update if this is a newer state and the type has actually changed
						if currentState == nil {
							c.logger.Debugf("New column state for %s.%s in file %s (type: %s)",
								tableName, colName, fileName, newType)

							// Clean any metadata values of trailing semicolons
							cleanMetadata := make(map[string]interface{})
							for k, v := range col.Metadata {
								if strVal, ok := v.(string); ok {
									cleanMetadata[k] = strings.TrimRight(strVal, ";")
								} else {
									cleanMetadata[k] = v
								}
							}
							col.Metadata = cleanMetadata

							tableStates[tableName][colName] = &ColumnState{
								Node:      col,
								Timestamp: ts,
								Type:      newType,
								LastFile:  fileName,
							}
						} else if ts > currentState.Timestamp && newType != currentState.Type {
							c.logger.Debugf("Updating column state for %s.%s from file %s (old_type: %s, new_type: %s)",
								tableName, colName, fileName, currentState.Type, newType)

							// Clean any metadata values of trailing semicolons
							cleanMetadata := make(map[string]interface{})
							for k, v := range col.Metadata {
								if strVal, ok := v.(string); ok {
									cleanMetadata[k] = strings.TrimRight(strVal, ";")
								} else {
									cleanMetadata[k] = v
								}
							}
							col.Metadata = cleanMetadata

							tableStates[tableName][colName] = &ColumnState{
								Node:      col,
								Timestamp: ts,
								Type:      newType,
								LastFile:  fileName,
							}
						} else {
							c.logger.Debugf("Skipping update for %s.%s (current_type: %s, new_type: %s, current_file: %s)",
								tableName, colName, currentState.Type, newType, currentState.LastFile)
						}
					}
				}

			case types.NodeIndex:
				indexName := strings.ToLower(node.Name)
				currentState := indexStates[indexName]

				// Only update if this is a newer state
				if currentState == nil {
					c.logger.Debugf("New index state for %s in file %s (table: %s, columns: %s)",
						indexName, fileName, node.Metadata["table"], node.Metadata["columns"])

					// Clean any metadata values of trailing semicolons
					cleanMetadata := make(map[string]interface{})
					for k, v := range node.Metadata {
						if strVal, ok := v.(string); ok {
							cleanMetadata[k] = strings.TrimRight(strVal, ";")
						} else {
							cleanMetadata[k] = v
						}
					}
					node.Metadata = cleanMetadata

					indexStates[indexName] = &IndexState{
						Node:      node,
						Timestamp: ts,
						LastFile:  fileName,
					}
				} else if ts > currentState.Timestamp {
					c.logger.Debugf("Updating index state for %s from file %s",
						indexName, fileName)

					// Clean any metadata values of trailing semicolons
					cleanMetadata := make(map[string]interface{})
					for k, v := range node.Metadata {
						if strVal, ok := v.(string); ok {
							cleanMetadata[k] = strings.TrimRight(strVal, ";")
						} else {
							cleanMetadata[k] = v
						}
					}
					node.Metadata = cleanMetadata

					indexStates[indexName] = &IndexState{
						Node:      node,
						Timestamp: ts,
						LastFile:  fileName,
					}
				} else {
					c.logger.Debugf("Skipping update for index %s (current_file: %s)",
						indexName, currentState.LastFile)
				}
			}
		}
	}

	// Build final tree from accumulated table and index states
	for tableName, columns := range tableStates {
		tableNode := &types.Node{
			Type:     types.NodeTable,
			Name:     tableName,
			Children: make([]*types.Node, 0),
			Metadata: make(map[string]interface{}),
		}

		// Add only the most recent state of each column
		for colName, state := range columns {
			tableNode.Children = append(tableNode.Children, state.Node)
			c.logger.Debugf("Final state for %s.%s: type=%s from file=%s",
				tableName, colName, state.Type, state.LastFile)
		}

		tree.Root.Children = append(tree.Root.Children, tableNode)
	}

	// Add indexes to the tree
	for indexName, state := range indexStates {
		tree.Root.Children = append(tree.Root.Children, state.Node)
		c.logger.Debugf("Final state for index %s: table=%s, columns=%s from file=%s",
			indexName, state.Node.Metadata["table"], state.Node.Metadata["columns"], state.LastFile)
	}

	return tree, nil
}

func (c *Collector) loadModuleSchema() (*types.SchemaTree, error) {
	tree := ast.NewSchemaTree()
	c.logger.Infof("Loading module schema files from: %s", c.modulesDir)

	// Track processed tables and indexes to avoid duplicates
	processedTables := make(map[string]bool)
	processedIndexes := make(map[string]bool)
	droppedTables := make(map[string]bool) // Track tables that should be dropped

	err := filepath.Walk(c.modulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".sql") {
			c.logger.Infof("Processing schema file: %s", path)
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", path, err)
			}

			sqlContent := string(content)
			parsed, err := c.parser.Parse(sqlContent)
			if err != nil {
				c.logger.Warnf("Failed to parse file %s: %v", path, err)
				return nil
			}

			// First pass: collect DROP TABLE statements
			statements := strings.Split(sqlContent, ";")
			for _, stmt := range statements {
				stmt = strings.TrimSpace(stmt)
				if strings.HasPrefix(strings.ToUpper(stmt), "DROP TABLE") {
					// Extract table name from DROP TABLE statement
					parts := strings.Fields(stmt)
					if len(parts) >= 3 {
						tableName := strings.ToLower(strings.TrimRight(parts[2], " \t\n\r;"))
						tableName = strings.TrimPrefix(tableName, "IF EXISTS ")
						tableName = strings.TrimSuffix(tableName, "CASCADE")
						tableName = strings.TrimSpace(tableName)
						droppedTables[tableName] = true
						c.logger.Debugf("Marked table for dropping: %s", tableName)
					}
				}
			}

			// Second pass: process CREATE and ALTER statements
			for _, node := range parsed.Root.Children {
				switch node.Type {
				case types.NodeTable:
					tableName := strings.ToLower(node.Name)

					// Skip if table is marked for dropping
					if droppedTables[tableName] {
						c.logger.Debugf("Skipping dropped table: %s", tableName)
						continue
					}

					// Skip if we've already processed this table
					if processedTables[tableName] {
						c.logger.Debugf("Skipping duplicate table: %s", node.Name)
						continue
					}
					processedTables[tableName] = true

					c.logger.Debugf("Found table: %s with %d columns", node.Name, len(node.Children))
					for _, col := range node.Children {
						if col.Type == types.NodeColumn {
							c.logger.Debugf("  Column: %s, Type: %s, Constraints: %s",
								col.Name,
								col.Metadata["type"],
								col.Metadata["constraints"])
						}
					}

					// Add table to tree
					tree.Root.Children = append(tree.Root.Children, node)
					c.logger.Debugf("Added table %s from %s", node.Name, path)

				case types.NodeIndex:
					indexName := strings.ToLower(node.Name)
					tableName := strings.ToLower(node.Metadata["table"].(string))

					// Skip if parent table is marked for dropping
					if droppedTables[tableName] {
						c.logger.Debugf("Skipping index for dropped table: %s", indexName)
						continue
					}

					// Skip if we've already processed this index
					if processedIndexes[indexName] {
						c.logger.Debugf("Skipping duplicate index: %s", node.Name)
						continue
					}
					processedIndexes[indexName] = true

					c.logger.Debugf("Found index: %s on table %s", node.Name, node.Metadata["table"])
					tree.Root.Children = append(tree.Root.Children, node)
					c.logger.Debugf("Added index %s from %s", node.Name, path)
				}
			}
		}
		return nil
	})

	// Log final state
	c.logger.Debug("Final module schema state:")
	for _, node := range tree.Root.Children {
		switch node.Type {
		case types.NodeTable:
			c.logger.Debugf("Table %s has %d columns", node.Name, len(node.Children))
			for _, col := range node.Children {
				if col.Type == types.NodeColumn {
					c.logger.Debugf("  Column: %s, Type: %s, Constraints: %s",
						col.Name,
						col.Metadata["type"],
						col.Metadata["constraints"])
				}
			}
		case types.NodeIndex:
			c.logger.Debugf("Index %s on table %s (columns: %s, unique: %v)",
				node.Name,
				node.Metadata["table"],
				node.Metadata["columns"],
				node.Metadata["is_unique"])
		}
	}

	return tree, err
}

// StoreMigrations writes detected changes to migration files
func (c *Collector) StoreMigrations(changes *diff.ChangeSet) error {
	if changes == nil || len(changes.Changes) == 0 {
		c.logger.Info("No changes to store")
		return nil
	}

	for _, change := range changes.Changes {
		c.logger.Debugf("Change details: Type=%s, Table=%s, Column=%s, ParentName=%s",
			change.Type, change.ObjectName, change.Object.Name, change.ParentName)
		if change.Object != nil && change.Object.Metadata != nil {
			c.logger.Debugf("Change metadata: %+v", change.Object.Metadata)
		}
	}

	generator, err := diff.NewGenerator(diff.GeneratorOptions{
		Dialect:        c.parser.GetDialect(),
		OutputDir:      c.baseDir,
		FileNameFormat: "changes-%d.sql",
		IncludeDown:    true,
		Logger:         c.logger,
	})
	if err != nil {
		c.logger.Errorf("Failed to create generator: %v", err)
		return fmt.Errorf("failed to create migration generator: %w", err)
	}

	c.logger.Debugf("Created generator with output dir: %s", c.baseDir)
	if err := generator.Generate(changes); err != nil {
		c.logger.Errorf("Error generating migrations: %v", err)
		return err
	}

	c.logger.Info("Finished")
	return nil
}
