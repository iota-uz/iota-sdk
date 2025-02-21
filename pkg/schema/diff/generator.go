package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/schema/dialect"
	"github.com/iota-uz/iota-sdk/pkg/schema/types"
	"github.com/sirupsen/logrus"
)

// Generator handles creation of migration files from detected changes
type Generator struct {
	dialect           dialect.Dialect
	outputDir         string
	templates         map[ChangeType]string
	options           GeneratorOptions
	logger            *logrus.Logger
	tableDependencies map[string][]string // tracks table -> dependencies
	processedTables   map[string]bool     // tracks which tables have been processed
	processedChanges  map[string]struct{} // tracks all processed objects by name
}

type GeneratorOptions struct {
	Dialect        string
	OutputDir      string
	FileNameFormat string
	IncludeDown    bool
	Logger         *logrus.Logger
	LogLevel       logrus.Level
}

// Generate creates migration files from a change set
func (g *Generator) Generate(changes *ChangeSet) error {
	if changes == nil || len(changes.Changes) == 0 {
		return nil
	}

	// Initialize tracking maps
	g.tableDependencies = make(map[string][]string)
	g.processedTables = make(map[string]bool)
	g.processedChanges = make(map[string]struct{})

	// First pass: build dependency graph and deduplicate tables
	deduplicatedChanges := make([]*Change, 0)

	// Process CREATE TABLE statements first
	for _, change := range changes.Changes {
		if change.Type == CreateTable {
			tableName := strings.ToLower(change.ObjectName)
			// Skip if we've already processed this table
			if _, exists := g.processedChanges[tableName]; exists {
				continue
			}
			g.processedChanges[tableName] = struct{}{}

			// Track dependencies
			for _, child := range change.Object.Children {
				if child.Type == types.NodeColumn {
					if refTable, ok := child.Metadata["referenced_table"].(string); ok && refTable != "" {
						refTable = strings.ToLower(refTable)
						g.tableDependencies[tableName] = append(g.tableDependencies[tableName], refTable)
					}
				}
			}
			deduplicatedChanges = append(deduplicatedChanges, change)
		}
	}

	// Process non-CREATE TABLE statements
	for _, change := range changes.Changes {
		if change.Type != CreateTable {
			objectKey := strings.ToLower(fmt.Sprintf("%s:%s", change.Type, change.ObjectName))
			if _, exists := g.processedChanges[objectKey]; exists {
				continue
			}
			g.processedChanges[objectKey] = struct{}{}
			deduplicatedChanges = append(deduplicatedChanges, change)
		}
	}

	// Sort changes based on dependencies
	sortedChanges := g.sortChangesByDependencies(deduplicatedChanges)
	changes.Changes = sortedChanges

	// Ensure output directory exists
	if err := os.MkdirAll(g.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create timestamp-based filename
	timestamp := time.Now().Unix()
	fileName := fmt.Sprintf("changes-%d.sql", timestamp)
	if g.options.FileNameFormat != "" {
		fileName = fmt.Sprintf(g.options.FileNameFormat, timestamp)
	}

	filePath := filepath.Join(g.outputDir, fileName)
	logger.Info("Generating migration file: ", filePath)

	var statements []string
	for _, change := range changes.Changes {
		stmt, err := g.generateChangeStatement(change)
		if err != nil {
			logger.Warnf("Error generating statement: %v", err)
			continue
		}
		if stmt != "" {
			logger.Debugf("Generated SQL: %s", stmt)
			statements = append(statements, stmt)
		}
	}

	if len(statements) == 0 {
		logger.Info("No statements generated")
		return nil
	}

	// Join statements with proper spacing and add migration marker
	var content strings.Builder
	content.WriteString("-- +migrate Up\n\n")
	for i, stmt := range statements {
		stmt = strings.TrimRight(stmt, ";") + ";"
		content.WriteString(stmt)
		if i < len(statements)-1 {
			content.WriteString("\n\n")
		}
	}

	// Write the migration file
	if err := os.WriteFile(filePath, []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("failed to write migration file: %w", err)
	}

	// Generate down migration if enabled
	if g.options.IncludeDown {
		downFileName := strings.Replace(fileName, ".sql", ".down.sql", 1)
		downFilePath := filepath.Join(g.outputDir, downFileName)

		downStatements := g.generateDownStatements(changes)
		if len(downStatements) > 0 {
			var downContent strings.Builder
			downContent.WriteString("-- +migrate Down\n\n")
			for i, stmt := range downStatements {
				stmt = strings.TrimRight(stmt, ";") + ";"
				downContent.WriteString(stmt)
				if i < len(downStatements)-1 {
					downContent.WriteString("\n\n")
				}
			}

			if err := os.WriteFile(downFilePath, []byte(downContent.String()), 0644); err != nil {
				return fmt.Errorf("failed to write down migration file: %w", err)
			}
		}
	}

	return nil
}

func (g *Generator) generateChangeStatement(change *Change) (string, error) {
	logger.Debugf("Generating statement for change type: %v", change.Type)

	switch change.Type {
	case CreateTable:
		logger.Debugf("Generating CREATE TABLE statement for %s", change.ObjectName)
		if originalSQL, ok := change.Object.Metadata["original_sql"].(string); ok && originalSQL != "" {
			logger.Debugf("Using original SQL for table %s: %s", change.ObjectName, originalSQL)
			return originalSQL, nil
		}
		var columns []string
		var constraints []string

		for _, child := range change.Object.Children {
			switch child.Type {
			case types.NodeColumn:
				if colDef := g.generateColumnDefinition(child); colDef != "" {
					columns = append(columns, "\t"+colDef)
				}
			case types.NodeConstraint:
				if def, ok := child.Metadata["definition"].(string); ok {
					// Ensure constraint definition ends with closing parenthesis if needed
					if strings.Count(def, "(") > strings.Count(def, ")") {
						def += ")"
					}
					constraints = append(constraints, "\t"+def)
				}
			}
		}

		var stmt strings.Builder
		stmt.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n", change.ObjectName))

		// Add columns
		if len(columns) > 0 {
			stmt.WriteString(strings.Join(columns, ",\n"))
		}

		// Add constraints
		if len(constraints) > 0 {
			if len(columns) > 0 {
				stmt.WriteString(",\n")
			}
			stmt.WriteString(strings.Join(constraints, ",\n"))
		}

		stmt.WriteString("\n);")
		return stmt.String(), nil

	case ModifyColumn:
		logger.Debugf("Generating ALTER COLUMN statement for %s.%s", change.ParentName, change.ObjectName)
		if def, ok := change.Object.Metadata["definition"].(string); ok {
			// Extract type and constraints from the definition
			parts := strings.SplitN(def, " ", 2)
			if len(parts) < 2 {
				return "", fmt.Errorf("invalid column definition: %s", def)
			}

			// Build ALTER COLUMN statement
			newType := change.Object.Metadata["fullType"].(string)
			stmt := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s",
				change.ParentName,
				change.ObjectName,
				newType)

			// Add nullability if it's changing
			constraints := change.Object.Metadata["constraints"].(string)
			if strings.Contains(strings.ToUpper(constraints), "NOT NULL") {
				stmt += " SET NOT NULL"
			} else if oldConstraints, ok := change.Metadata["old_constraints"].(string); ok &&
				strings.Contains(strings.ToUpper(oldConstraints), "NOT NULL") {
				stmt += " DROP NOT NULL"
			}

			// Add default value if present
			if strings.Contains(strings.ToUpper(constraints), "DEFAULT") {
				defaultValue := extractDefaultValue(constraints)
				if defaultValue != "" {
					stmt += fmt.Sprintf(" SET DEFAULT %s", defaultValue)
				}
			}

			return stmt, nil
		}
		return "", fmt.Errorf("missing column definition for %s", change.ObjectName)

	case AddColumn:
		logger.Debugf("Generating ADD COLUMN statement for %s.%s", change.ParentName, change.ObjectName)
		logger.Debugf("Column metadata: %+v", change.Object.Metadata)

		if def, ok := change.Object.Metadata["definition"].(string); ok {
			stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;",
				change.ParentName,
				def)
			logger.Debugf("Generated statement: %s", stmt)
			return stmt, nil
		}

		// Fallback with proper semicolon
		rawType, ok := change.Object.Metadata["rawType"].(string)
		if !ok {
			return "", fmt.Errorf("missing raw type for column %s", change.ObjectName)
		}

		stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;",
			change.ParentName,
			change.ObjectName,
			rawType)
		logger.Debugf("Generated fallback statement: %s", stmt)
		return stmt, nil

	case AddIndex:
		logger.Debugf("Generating CREATE INDEX statement for %s", change.ObjectName)
		if originalSQL, ok := change.Object.Metadata["original_sql"].(string); ok && originalSQL != "" {
			logger.Debugf("Using original SQL for index %s: %s", change.ObjectName, originalSQL)
			return originalSQL + ";", nil
		}
		// Fallback to constructing the index statement
		isUnique := change.Object.Metadata["is_unique"].(bool)
		tableName := change.Object.Metadata["table"].(string)
		columns := change.Object.Metadata["columns"].(string)

		var stmt strings.Builder
		stmt.WriteString("CREATE ")
		if isUnique {
			stmt.WriteString("UNIQUE ")
		}
		stmt.WriteString(fmt.Sprintf("INDEX %s ON %s (%s);",
			change.ObjectName, tableName, columns))

		result := stmt.String()
		logger.Debugf("Generated index statement: %s", result)
		return result, nil

	case ModifyIndex:
		logger.Debugf("Generating MODIFY INDEX statement for %s", change.ObjectName)
		// For index modifications, we drop and recreate
		if newDef, ok := change.Metadata["new_definition"].(string); ok {
			dropStmt := fmt.Sprintf("DROP INDEX IF EXISTS %s;", change.ObjectName)
			result := dropStmt + "\n" + newDef + ";"
			logger.Debugf("Generated index modification statement: %s", result)
			return result, nil
		}
		return "", fmt.Errorf("missing new index definition for %s", change.ObjectName)

	case DropIndex:
		logger.Debugf("Generating DROP INDEX statement for %s", change.ObjectName)
		return fmt.Sprintf("DROP INDEX IF EXISTS %s;", change.ObjectName), nil

	case AddConstraint:
		if def, ok := change.Object.Metadata["definition"].(string); ok {
			return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s %s;",
				change.ObjectName, change.Object.Name, def), nil
		}
	}

	return "", fmt.Errorf("unsupported change type or missing data: %v", change.Type)
}

func extractDefaultValue(constraints string) string {
	defaultIdx := strings.Index(strings.ToUpper(constraints), "DEFAULT")
	if defaultIdx == -1 {
		return ""
	}

	// Extract everything after DEFAULT
	defaultPart := strings.TrimSpace(constraints[defaultIdx+7:])

	// Handle quoted values
	if strings.HasPrefix(defaultPart, "'") {
		endQuote := strings.Index(defaultPart[1:], "'")
		if endQuote != -1 {
			return defaultPart[:endQuote+2]
		}
	}

	// Handle non-quoted values (stop at first space or comma)
	endIdx := strings.IndexAny(defaultPart, " ,")
	if endIdx == -1 {
		return defaultPart
	}
	return defaultPart[:endIdx]
}

func (g *Generator) generateDownStatements(changes *ChangeSet) []string {
	logger.Debugf("Generating down statements for %d changes", len(changes.Changes))
	// Generate reverse operations in reverse order
	statements := make([]string, 0, len(changes.Changes))
	for i := len(changes.Changes) - 1; i >= 0; i-- {
		change := changes.Changes[i]
		if !change.Reversible {
			logger.Debugf("Skipping non-reversible change: %v", change.Type)
			continue
		}

		stmt := g.generateDownStatement(change)
		if stmt != "" {
			logger.Debugf("Generated down statement: %s", stmt)
			statements = append(statements, stmt)
		}
	}
	return statements
}

func (g *Generator) generateDownStatement(change *Change) string {
	logger.Debugf("Generating down statement for change type: %v", change.Type)

	switch change.Type {
	case CreateTable:
		stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s;", change.ObjectName)
		logger.Debugf("Generated down statement for table: %s", stmt)
		return stmt
	case AddColumn:
		stmt := fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s;", change.ParentName, change.ObjectName)
		logger.Debugf("Generated down statement for column: %s", stmt)
		return stmt
	case AddConstraint:
		stmt := fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;", change.ParentName, change.ObjectName)
		logger.Debugf("Generated down statement for constraint: %s", stmt)
		return stmt
	case AddIndex, ModifyIndex:
		stmt := fmt.Sprintf("DROP INDEX IF EXISTS %s;", change.ObjectName)
		logger.Debugf("Generated down statement for index: %s", stmt)
		return stmt
	}
	return ""
}

// NewGenerator creates a new migration generator
func NewGenerator(opts GeneratorOptions) (*Generator, error) {
	d, ok := dialect.Get(opts.Dialect)
	if !ok {
		return nil, fmt.Errorf("unsupported dialect: %s", opts.Dialect)
	}

	return &Generator{
		dialect:   d,
		outputDir: opts.OutputDir,
		options:   opts,
		templates: loadDefaultTemplates(),
		logger:    logger,
	}, nil
}

func loadDefaultTemplates() map[ChangeType]string {
	return map[ChangeType]string{
		AddColumn:     `ALTER TABLE {{ .TableName }} ADD COLUMN {{ .ColumnDef }}`,
		AddConstraint: `ALTER TABLE {{ .TableName }} ADD CONSTRAINT {{ .ConstraintName }} {{ .ConstraintDef }}`,
		AddIndex:      `CREATE INDEX {{ .IndexName }} ON {{ .TableName }} {{ .IndexDef }}`,
	}
}

func (g *Generator) generateColumnDefinition(col *types.Node) string {
	if col == nil {
		return ""
	}

	if def, ok := col.Metadata["definition"].(string); ok {
		// Ensure definition ends with closing parenthesis if it has an opening one
		if strings.Count(def, "(") > strings.Count(def, ")") {
			def += ")"
		}
		return def
	}

	var b strings.Builder
	b.WriteString(col.Name)
	b.WriteString(" ")

	if typeVal, ok := col.Metadata["type"].(string); ok {
		if mappedType, exists := g.dialect.GetDataTypeMapping()[strings.ToLower(typeVal)]; exists {
			b.WriteString(mappedType)
		} else {
			b.WriteString(typeVal)
		}

		// Add closing parenthesis if type definition has an opening one
		if strings.Contains(typeVal, "(") && !strings.Contains(typeVal, ")") {
			b.WriteString(")")
		}
	}

	if constraints, ok := col.Metadata["constraints"].(string); ok && constraints != "" {
		b.WriteString(" ")
		// Ensure constraints end with closing parenthesis if needed
		if strings.Count(constraints, "(") > strings.Count(constraints, ")") {
			constraints += ")"
		}
		b.WriteString(constraints)
	}

	return strings.TrimSpace(b.String())
}

// Add new method to sort changes based on dependencies
func (g *Generator) sortChangesByDependencies(changes []*Change) []*Change {
	tableCreations := make(map[string]*Change)
	var otherChanges []*Change

	// Create map of table creations, using lowercase names for consistency
	for _, change := range changes {
		if change.Type == CreateTable {
			tableName := strings.ToLower(change.ObjectName)
			// Only include each table once
			if _, exists := tableCreations[tableName]; !exists {
				tableCreations[tableName] = change
			}
		} else {
			otherChanges = append(otherChanges, change)
		}
	}

	// Perform topological sort
	var sorted []*Change
	visited := make(map[string]bool)
	visiting := make(map[string]bool)

	var visit func(string) error
	visit = func(table string) error {
		tableLower := strings.ToLower(table)
		if visiting[tableLower] {
			return fmt.Errorf("circular dependency detected involving table %s", table)
		}
		if visited[tableLower] {
			return nil
		}

		visiting[tableLower] = true
		// Process dependencies first
		for _, dep := range g.tableDependencies[tableLower] {
			depLower := strings.ToLower(dep)
			if change, exists := tableCreations[depLower]; exists {
				if err := visit(depLower); err != nil {
					return err
				}
				// Only add if not already in sorted list
				if !g.isTableInList(change.ObjectName, sorted) {
					sorted = append(sorted, change)
				}
			}
		}
		visiting[tableLower] = false
		visited[tableLower] = true

		// Add the current table if not already in sorted list
		if change, exists := tableCreations[tableLower]; exists {
			if !g.isTableInList(change.ObjectName, sorted) {
				sorted = append(sorted, change)
			}
		}
		return nil
	}

	// Visit all tables
	for tableName := range tableCreations {
		if !visited[tableName] {
			if err := visit(tableName); err != nil {
				g.logger.Warnf("Dependency resolution error: %v", err)
				// Fall back to original order if there's an error
				return changes
			}
		}
	}

	// Append non-table changes at the end
	sorted = append(sorted, otherChanges...)
	return sorted
}

// Helper function to check if a table is already in the sorted list
func (g *Generator) isTableInList(tableName string, list []*Change) bool {
	tableLower := strings.ToLower(tableName)
	for _, change := range list {
		if change.Type == CreateTable && strings.ToLower(change.ObjectName) == tableLower {
			return true
		}
	}
	return false
}
