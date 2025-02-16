package collector

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/schema/ast"
	"github.com/iota-uz/iota-sdk/pkg/schema/dialect"
	"github.com/iota-uz/iota-sdk/pkg/schema/diff"
	"github.com/iota-uz/iota-sdk/pkg/schema/types"
)

// Collector handles collecting and analyzing migrations from modules
type Collector struct {
	baseDir    string
	modulesDir string
	parser     *ast.Parser
	migrations map[string]*types.SchemaTree
	dialect    dialect.Dialect
}

// Config holds collector configuration
type Config struct {
	ModulesPath    string
	MigrationsPath string
	SQLDialect     string
}

// New creates a new migration collector
func New(cfg Config) *Collector {
	d, ok := dialect.Get(cfg.SQLDialect)
	if !ok {
		d = dialect.NewPostgresDialect() // Default to PostgreSQL
	}

	return &Collector{
		baseDir:    cfg.MigrationsPath,
		modulesDir: cfg.ModulesPath,
		parser:     ast.NewParser(cfg.SQLDialect, ast.ParserOptions{StrictMode: true}),
		migrations: make(map[string]*types.SchemaTree),
		dialect:    d,
	}
}

// CollectMigrations gathers all migrations from modules and analyzes changes
func (c *Collector) CollectMigrations(ctx context.Context) (*diff.ChangeSet, error) {
	log.Printf("Starting CollectMigrations")

	oldTree, err := c.loadExistingSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to load existing schema: %w", err)
	}
	log.Printf("Loaded existing schema with %d tables", len(oldTree.Root.Children))
	for _, node := range oldTree.Root.Children {
		if node.Type == types.NodeTable {
			log.Printf("Existing schema table: %s with %d columns", node.Name, len(node.Children))
		}
	}

	newTree, err := c.loadModuleSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to load module schema: %w", err)
	}
	log.Printf("Loaded module schema with %d tables", len(newTree.Root.Children))
	for _, node := range newTree.Root.Children {
		if node.Type == types.NodeTable {
			log.Printf("Module schema table: %s with %d columns", node.Name, len(node.Children))
		}
	}

	log.Printf("Creating analyzer for schema comparison")
	analyzer := diff.NewAnalyzer(oldTree, newTree, diff.AnalyzerOptions{
		IgnoreCase:          true,
		IgnoreWhitespace:    true,
		DetectRenames:       true,
		ValidateConstraints: true,
	})

	log.Printf("Starting schema comparison")
	changes, err := analyzer.Compare()
	if err != nil {
		log.Printf("Error during comparison: %v", err)
		return nil, err
	}
	log.Printf("Completed schema comparison, found %d changes", len(changes.Changes))

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
	log.Printf("Loading existing schema from: %s", c.baseDir)

	// Read migration files
	files, err := os.ReadDir(c.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("No existing migrations directory found at: %s", c.baseDir)
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

	// Track the latest state of each column with its type and timestamp
	type ColumnState struct {
		Node      *types.Node
		Timestamp int64
		Type      string
		LastFile  string
	}
	tableStates := make(map[string]map[string]*ColumnState) // table -> column -> state

	// Process migrations in chronological order
	for _, fileName := range migrationFiles {
		log.Printf("Processing migration file: %s", fileName)
		timestamp := strings.TrimSuffix(strings.TrimPrefix(fileName, "changes-"), ".sql")
		ts, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			log.Printf("Warning: invalid timestamp in filename %s: %v", fileName, err)
			continue
		}

		path := filepath.Join(c.baseDir, fileName)
		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Warning: failed to read file %s: %v", path, err)
			continue
		}

		sql := string(content)
		parsed, err := c.parser.Parse(sql)
		if err != nil {
			log.Printf("Warning: failed to parse file %s: %v", path, err)
			continue
		}

		// Update table states with changes from this migration
		for _, node := range parsed.Root.Children {
			if node.Type == types.NodeTable {
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
							newType = strings.ToLower(fullType)
						} else if typeStr, ok := col.Metadata["type"].(string); ok {
							newType = strings.ToLower(typeStr)
						}

						// Only update if this is a newer state and the type has actually changed
						if currentState == nil {
							log.Printf("New column state for %s.%s in file %s (type: %s)",
								tableName, colName, fileName, newType)
							tableStates[tableName][colName] = &ColumnState{
								Node:      col,
								Timestamp: ts,
								Type:      newType,
								LastFile:  fileName,
							}
						} else if ts > currentState.Timestamp && newType != currentState.Type {
							log.Printf("Updating column state for %s.%s from file %s (old_type: %s, new_type: %s)",
								tableName, colName, fileName, currentState.Type, newType)
							tableStates[tableName][colName] = &ColumnState{
								Node:      col,
								Timestamp: ts,
								Type:      newType,
								LastFile:  fileName,
							}
						} else {
							log.Printf("Skipping update for %s.%s (current_type: %s, new_type: %s, current_file: %s)",
								tableName, colName, currentState.Type, newType, currentState.LastFile)
						}
					}
				}
			}
		}
	}

	// Build final tree from accumulated table states
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
			log.Printf("Final state for %s.%s: type=%s from file=%s",
				tableName, colName, state.Type, state.LastFile)
		}

		tree.Root.Children = append(tree.Root.Children, tableNode)
	}

	return tree, nil
}

func (c *Collector) loadModuleSchema() (*types.SchemaTree, error) {
	tree := ast.NewSchemaTree()
	log.Printf("Loading module schema from: %s", c.modulesDir)

	// Track processed tables to avoid duplicates
	processedTables := make(map[string]bool)

	err := filepath.Walk(c.modulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, "-schema.sql") {
			log.Printf("Processing schema file: %s", path)
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", path, err)
			}

			sqlContent := string(content)
			parsed, err := c.parser.Parse(sqlContent)
			if err != nil {
				log.Printf("Warning: failed to parse file %s: %v", path, err)
				return nil
			}

			// Log found tables and columns
			for _, node := range parsed.Root.Children {
				if node.Type == types.NodeTable {
					tableName := strings.ToLower(node.Name)
					log.Printf("Found table: %s with %d columns", node.Name, len(node.Children))

					// Skip if we've already processed this table
					if processedTables[tableName] {
						log.Printf("Skipping duplicate table: %s", node.Name)
						continue
					}
					processedTables[tableName] = true

					for _, col := range node.Children {
						if col.Type == types.NodeColumn {
							log.Printf("  Column: %s, Type: %s, Constraints: %s",
								col.Name,
								col.Metadata["type"],
								col.Metadata["constraints"])
						}
					}

					// Add table to tree
					tree.Root.Children = append(tree.Root.Children, node)
					log.Printf("Added table %s from %s", node.Name, path)
				}
			}
		}
		return nil
	})

	// Log final state
	log.Printf("Final module schema state:")
	for _, node := range tree.Root.Children {
		if node.Type == types.NodeTable {
			log.Printf("Table %s has %d columns", node.Name, len(node.Children))
			for _, col := range node.Children {
				if col.Type == types.NodeColumn {
					log.Printf("  Column: %s, Type: %s, Constraints: %s",
						col.Name,
						col.Metadata["type"],
						col.Metadata["constraints"])
				}
			}
		}
	}

	return tree, err
}

// StoreMigrations writes detected changes to migration files
func (c *Collector) StoreMigrations(changes *diff.ChangeSet) error {
	if changes == nil || len(changes.Changes) == 0 {
		log.Printf("No changes to store")
		return nil
	}

	log.Printf("Found %d changes to store", len(changes.Changes))
	for _, change := range changes.Changes {
		log.Printf("Change details: Type=%s, Table=%s, Column=%s, ParentName=%s",
			change.Type, change.ObjectName, change.Object.Name, change.ParentName)
		if change.Object != nil && change.Object.Metadata != nil {
			log.Printf("Change metadata: %+v", change.Object.Metadata)
		}
	}

	generator, err := diff.NewGenerator(diff.GeneratorOptions{
		Dialect:        c.parser.GetDialect(),
		OutputDir:      c.baseDir,
		FileNameFormat: "changes-%d.sql",
		IncludeDown:    true,
	})
	if err != nil {
		log.Printf("Failed to create generator: %v", err)
		return fmt.Errorf("failed to create migration generator: %w", err)
	}

	log.Printf("Created generator with output dir: %s", c.baseDir)
	if err := generator.Generate(changes); err != nil {
		log.Printf("Error generating migrations: %v", err)
		return err
	}

	log.Printf("Successfully generated migration files")
	return nil
}

func (c *Collector) mergeTableNodes(existing, new *types.Node) *types.Node {
	// Create a new node to avoid modifying the original
	merged := &types.Node{
		Type:     types.NodeTable,
		Name:     existing.Name,
		Children: make([]*types.Node, 0),
		Metadata: make(map[string]interface{}),
	}

	// Copy metadata from the new node
	for k, v := range new.Metadata {
		merged.Metadata[k] = v
	}

	// Create a map of existing columns and constraints
	existingChildren := make(map[string]*types.Node)
	for _, child := range existing.Children {
		existingChildren[strings.ToLower(child.Name)] = child
	}

	// Process new children, overwriting or adding as needed
	for _, child := range new.Children {
		merged.Children = append(merged.Children, child)
		delete(existingChildren, strings.ToLower(child.Name))
	}

	// Add remaining existing children that weren't overwritten
	for _, child := range existingChildren {
		merged.Children = append(merged.Children, child)
	}

	return merged
}
