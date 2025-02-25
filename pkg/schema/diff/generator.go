package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/schema/dialect"
	"github.com/iota-uz/iota-sdk/pkg/schema/types"
	"github.com/sirupsen/logrus"
)

// Generator handles creation of migration files from detected changes
type Generator struct {
	dialect   dialect.Dialect
	outputDir string
	templates map[ChangeType]string
	options   GeneratorOptions
	logger    *logrus.Logger
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

	// Detect potential column renames
	var renamedColumns map[string]string
	defer func() {
		if r := recover(); r != nil {
			g.logger.Errorf("Recovered from panic in detectColumnRenames: %v", r)
			renamedColumns = make(map[string]string)
		}
	}()

	renamedColumns = g.detectColumnRenames(changes)
	g.logger.Debugf("Detected %d potential column renames", len(renamedColumns))

	// Check existing migrations for tables and columns that have already been dropped
	existingDropped, err := g.CheckExistingMigrations()
	if err != nil {
		g.logger.Warnf("Failed to check existing migrations: %v", err)
		// Continue with empty map if we can't check existing migrations
		existingDropped = make(map[string]bool)
	}

	// Filter out duplicate changes for tables that have already been dropped
	filteredChanges := make([]*Change, 0, len(changes.Changes))
	droppedTables := make(map[string]bool)
	droppedColumns := make(map[string]bool) // Format: "tableName.columnName"

	// First pass: identify all tables and columns being dropped in this changeset
	for _, change := range changes.Changes {
		if change.Type == DropTable {
			droppedTables[change.ObjectName] = true
		} else if change.Type == DropColumn {
			key := fmt.Sprintf("%s.%s", change.ParentName, change.ObjectName)
			droppedColumns[key] = true
		}
	}

	// Second pass: filter changes
	for _, change := range changes.Changes {
		// If this change is part of a rename operation, always include it
		if change.Metadata != nil && change.Metadata["is_rename_part"] == true {
			g.logger.Debugf("Including change for %s as part of a column rename operation", change.ObjectName)
			filteredChanges = append(filteredChanges, change)
			continue
		}

		// Skip duplicate DROP TABLE operations
		if change.Type == DropTable {
			// Skip if the table has already been dropped in a previous migration
			if existingDropped[change.ObjectName] {
				g.logger.Infof("Skipping DROP TABLE for %s as it was already dropped in a previous migration", change.ObjectName)
				continue
			}

			// Skip if we've already included this table in the current changeset
			if droppedTables[change.ObjectName] && change != changes.Changes[0] {
				g.logger.Debugf("Skipping duplicate DROP TABLE for %s in current changeset", change.ObjectName)
				continue
			}

			// If we get here, this is a valid DROP TABLE operation
			g.logger.Infof("Including DROP TABLE for %s", change.ObjectName)
		}

		// Skip DROP COLUMN operations for tables that have been or will be dropped
		if change.Type == DropColumn {
			// Skip if the parent table has already been dropped in a previous migration
			if existingDropped[change.ParentName] {
				g.logger.Debugf("Skipping DROP COLUMN %s for table %s as the table was already dropped in a previous migration",
					change.ObjectName, change.ParentName)
				continue
			}

			// Skip if the parent table is being dropped in this changeset
			if droppedTables[change.ParentName] {
				g.logger.Debugf("Skipping DROP COLUMN %s for table %s as the table is being dropped in this changeset",
					change.ObjectName, change.ParentName)
				continue
			}

			// Skip if this column has already been dropped in a previous migration
			columnKey := fmt.Sprintf("%s.%s", change.ParentName, change.ObjectName)
			if existingDropped[columnKey] {
				g.logger.Debugf("Skipping DROP COLUMN %s for table %s as it was already dropped in a previous migration",
					change.ObjectName, change.ParentName)
				continue
			}

			// Skip if we've already included this column drop in the current changeset
			if droppedColumns[columnKey] && change != changes.Changes[0] {
				g.logger.Debugf("Skipping duplicate DROP COLUMN %s for table %s in current changeset",
					change.ObjectName, change.ParentName)
				continue
			}
		}

		// Skip ADD COLUMN operations for tables that have been dropped in previous migrations
		// or for columns that have already been dropped
		if change.Type == AddColumn {
			// Skip if the parent table has already been dropped in a previous migration
			if existingDropped[change.ParentName] {
				g.logger.Debugf("Skipping ADD COLUMN %s for table %s as the table was already dropped in a previous migration",
					change.ObjectName, change.ParentName)
				continue
			}

			// Skip if this specific column has already been dropped in a previous migration
			columnKey := fmt.Sprintf("%s.%s", change.ParentName, change.ObjectName)
			if existingDropped[columnKey] {
				g.logger.Debugf("Skipping ADD COLUMN %s for table %s as this column was already dropped in a previous migration",
					change.ObjectName, change.ParentName)
				continue
			}
		}

		filteredChanges = append(filteredChanges, change)
	}

	// If all changes were filtered out, return early
	if len(filteredChanges) == 0 {
		g.logger.Info("No new changes to apply")
		return nil
	}

	// Update the changes in the changeset
	changes.Changes = filteredChanges

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
	g.logger.Info("Generating migration file: ", filePath)

	var statements []string
	for _, change := range changes.Changes {
		stmt, err := g.generateChangeStatement(change)
		if err != nil {
			g.logger.Warnf("Error generating statement: %v", err)
			continue
		}
		if stmt != "" {
			g.logger.Debugf("Generated SQL: %s", stmt)
			statements = append(statements, stmt)
		}
	}

	if len(statements) == 0 {
		g.logger.Info("No statements generated")
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
	g.logger.Debugf("Generating statement for change type: %v", change.Type)

	switch change.Type {
	case CreateTable:
		g.logger.Debugf("Generating CREATE TABLE statement for %s", change.ObjectName)
		if originalSQL, ok := change.Object.Metadata["original_sql"].(string); ok && originalSQL != "" {
			g.logger.Debugf("Using original SQL for table %s: %s", change.ObjectName, originalSQL)
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

	case DropTable:
		g.logger.Debugf("Generating DROP TABLE statement for %s", change.ObjectName)
		// Check if we have the original definition for down migration
		if g.options.IncludeDown && change.Metadata["original_create_sql"] == nil {
			g.logger.Debugf("No original definition available for table %s, marking as non-reversible", change.ObjectName)
			change.Reversible = false
		}
		return fmt.Sprintf("DROP TABLE IF EXISTS %s;", change.ObjectName), nil

	case DropColumn:
		g.logger.Debugf("Generating DROP COLUMN statement for %s.%s", change.ParentName, change.ObjectName)

		// Check if this is part of a rename operation
		if change.Metadata != nil {
			if renamedTo, ok := change.Metadata["renamed_to"].(string); ok && renamedTo != "" {
				g.logger.Debugf("Column %s.%s is being renamed to %s - generating DROP part of rename",
					change.ParentName, change.ObjectName, renamedTo)

				// For renames, we still need to drop the old column
				// Store original column definition for down migration if available
				if g.options.IncludeDown {
					if originalDef, ok := change.Metadata["original_definition"].(string); !ok || originalDef == "" {
						g.logger.Debugf("No original definition available for column %s, marking as non-reversible", change.ObjectName)
						change.Reversible = false
					}
				}
			}
		} else {
			// Regular drop column operation
			// Store original column definition for down migration if available
			if g.options.IncludeDown {
				if change.Metadata != nil {
					if originalDef, ok := change.Metadata["original_definition"].(string); !ok || originalDef == "" {
						g.logger.Debugf("No original definition available for column %s, marking as non-reversible", change.ObjectName)
						change.Reversible = false
					}
				} else {
					g.logger.Debugf("No metadata available for column %s, marking as non-reversible", change.ObjectName)
					change.Reversible = false
				}
			}
		}

		return fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s;", change.ParentName, change.ObjectName), nil

	case ModifyColumn:
		g.logger.Debugf("Generating ALTER COLUMN statement for %s.%s", change.ParentName, change.ObjectName)
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
		g.logger.Debugf("Generating ADD COLUMN statement for %s.%s", change.ParentName, change.ObjectName)

		// Check if this is part of a rename operation
		if change.Metadata != nil {
			if renamedFrom, ok := change.Metadata["renamed_from"].(string); ok && renamedFrom != "" {
				g.logger.Debugf("Column %s.%s is renamed from %s - generating ADD part of rename",
					change.ParentName, change.ObjectName, renamedFrom)

				// For renames, we still generate a regular ADD COLUMN statement
				// The DROP COLUMN for the old name will be handled separately
			}
		}

		// Safety check for nil Object
		if change.Object == nil {
			return "", fmt.Errorf("missing object data for column %s", change.ObjectName)
		}

		// Safety check for nil Metadata
		if change.Object.Metadata == nil {
			return "", fmt.Errorf("missing metadata for column %s", change.ObjectName)
		}

		g.logger.Debugf("Column metadata: %+v", change.Object.Metadata)

		if def, ok := change.Object.Metadata["definition"].(string); ok && def != "" {
			stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;",
				change.ParentName,
				def)
			g.logger.Debugf("Generated statement: %s", stmt)
			return stmt, nil
		}

		// Fallback with proper semicolon
		rawType, ok := change.Object.Metadata["rawType"].(string)
		if !ok || rawType == "" {
			return "", fmt.Errorf("missing raw type for column %s", change.ObjectName)
		}

		stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;",
			change.ParentName,
			change.ObjectName,
			rawType)
		g.logger.Debugf("Generated fallback statement: %s", stmt)
		return stmt, nil

	case AddIndex:
		g.logger.Debugf("Generating CREATE INDEX statement for %s", change.ObjectName)
		if originalSQL, ok := change.Object.Metadata["original_sql"].(string); ok && originalSQL != "" {
			g.logger.Debugf("Using original SQL for index %s: %s", change.ObjectName, originalSQL)
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
		g.logger.Debugf("Generated index statement: %s", result)
		return result, nil

	case ModifyIndex:
		g.logger.Debugf("Generating MODIFY INDEX statement for %s", change.ObjectName)
		// For index modifications, we drop and recreate
		if newDef, ok := change.Metadata["new_definition"].(string); ok {
			dropStmt := fmt.Sprintf("DROP INDEX IF EXISTS %s;", change.ObjectName)
			result := dropStmt + "\n" + newDef + ";"
			g.logger.Debugf("Generated index modification statement: %s", result)
			return result, nil
		}
		return "", fmt.Errorf("missing new index definition for %s", change.ObjectName)

	case DropIndex:
		g.logger.Debugf("Generating DROP INDEX statement for %s", change.ObjectName)
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
	g.logger.Debugf("Generating down statements for %d changes", len(changes.Changes))
	// Generate reverse operations in reverse order
	statements := make([]string, 0, len(changes.Changes))
	for i := len(changes.Changes) - 1; i >= 0; i-- {
		change := changes.Changes[i]
		if !change.Reversible {
			g.logger.Debugf("Skipping non-reversible change: %v", change.Type)
			continue
		}

		stmt := g.generateDownStatement(change)
		if stmt != "" {
			g.logger.Debugf("Generated down statement: %s", stmt)
			statements = append(statements, stmt)
		}
	}
	return statements
}

func (g *Generator) generateDownStatement(change *Change) string {
	g.logger.Debugf("Generating down statement for change type: %v", change.Type)

	switch change.Type {
	case CreateTable:
		stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s;", change.ObjectName)
		g.logger.Debugf("Generated down statement for table: %s", stmt)
		return stmt
	case DropTable:
		// For drop table, the down statement would be to recreate the table
		// This is complex and requires the original table definition
		if change.Metadata != nil {
			if originalSQL, ok := change.Metadata["original_create_sql"].(string); ok && originalSQL != "" {
				g.logger.Debugf("Using original SQL to recreate table %s", change.ObjectName)
				return originalSQL
			}
		}
		// We should never reach here as we mark non-reversible changes in generateChangeStatement
		g.logger.Debugf("Skipping down statement for DROP TABLE %s as it was marked non-reversible", change.ObjectName)
		return ""
	case DropColumn:
		// For drop column, the down statement would be to add the column back
		if change.Metadata != nil {
			// Check if this is part of a rename operation
			if renamedTo, ok := change.Metadata["renamed_to"].(string); ok && renamedTo != "" {
				g.logger.Debugf("Column %s.%s was renamed to %s, generating special down statement",
					change.ParentName, change.ObjectName, renamedTo)

				// For renames, the down statement should rename the column back
				if originalDef, ok := change.Metadata["original_definition"].(string); ok && originalDef != "" {
					stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", change.ParentName, originalDef)
					g.logger.Debugf("Generated down statement for renamed column: %s", stmt)
					return stmt
				}
			}

			// Regular drop column
			if originalDef, ok := change.Metadata["original_definition"].(string); ok && originalDef != "" {
				stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", change.ParentName, originalDef)
				g.logger.Debugf("Generated down statement for column: %s", stmt)
				return stmt
			}
		}
		// We should never reach here as we mark non-reversible changes in generateChangeStatement
		g.logger.Debugf("Skipping down statement for DROP COLUMN %s as it was marked non-reversible", change.ObjectName)
		return ""
	case AddColumn:
		// Check if this is part of a rename operation
		if change.Metadata != nil && change.Metadata["renamed_from"] != nil {
			renamedFrom, ok := change.Metadata["renamed_from"].(string)
			if ok && renamedFrom != "" {
				g.logger.Debugf("Column %s.%s was renamed from %s, generating special down statement",
					change.ParentName, change.ObjectName, renamedFrom)

				// For renames, the down statement should drop this column and add back the original
				stmt := fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s;", change.ParentName, change.ObjectName)
				g.logger.Debugf("Generated down statement for renamed column: %s", stmt)
				return stmt
			}
		}

		// Regular add column
		stmt := fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s;", change.ParentName, change.ObjectName)
		g.logger.Debugf("Generated down statement for column: %s", stmt)
		return stmt
	case AddConstraint:
		stmt := fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;", change.ParentName, change.ObjectName)
		g.logger.Debugf("Generated down statement for constraint: %s", stmt)
		return stmt
	case AddIndex, ModifyIndex:
		stmt := fmt.Sprintf("DROP INDEX IF EXISTS %s;", change.ObjectName)
		g.logger.Debugf("Generated down statement for index: %s", stmt)
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

	// Initialize logger if not provided
	logger := opts.Logger
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(opts.LogLevel)
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
		DropColumn:    `ALTER TABLE {{ .TableName }} DROP COLUMN IF EXISTS {{ .ColumnName }}`,
		AddConstraint: `ALTER TABLE {{ .TableName }} ADD CONSTRAINT {{ .ConstraintName }} {{ .ConstraintDef }}`,
		AddIndex:      `CREATE INDEX {{ .IndexName }} ON {{ .TableName }} {{ .IndexDef }}`,
		DropTable:     `DROP TABLE IF EXISTS {{ .TableName }}`,
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

// CheckExistingMigrations scans existing migration files for DROP TABLE and DROP COLUMN statements
func (g *Generator) CheckExistingMigrations() (map[string]bool, error) {
	droppedTables := make(map[string]bool)
	droppedColumns := make(map[string]bool) // Format: "tableName.columnName"

	// Read all SQL files in the output directory
	files, err := os.ReadDir(g.outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return droppedTables, nil
		}
		return nil, fmt.Errorf("failed to read migration directory: %w", err)
	}

	// Sort files by name to process them in chronological order
	// This ensures we correctly track the most recent state
	fileNames := make([]string, 0, len(files))
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") && !strings.Contains(file.Name(), ".down.sql") {
			fileNames = append(fileNames, file.Name())
		}
	}
	// Sort filenames (typically timestamp-based)
	sort.Strings(fileNames)

	// Regex patterns to match DROP statements
	dropTablePattern := `DROP\s+TABLE\s+IF\s+EXISTS\s+([a-zA-Z0-9_]+)`
	dropColumnPattern := `ALTER\s+TABLE\s+([a-zA-Z0-9_]+)\s+DROP\s+COLUMN\s+IF\s+EXISTS\s+([a-zA-Z0-9_]+)`
	createTablePattern := `CREATE\s+TABLE\s+IF\s+NOT\s+EXISTS\s+([a-zA-Z0-9_]+)`

	for _, fileName := range fileNames {
		filePath := filepath.Join(g.outputDir, fileName)
		content, err := os.ReadFile(filePath)
		if err != nil {
			g.logger.Warnf("Failed to read migration file %s: %v", fileName, err)
			continue
		}

		contentStr := string(content)

		// Look for CREATE TABLE statements - these would "undo" previous DROP TABLE operations
		createMatches := regexp.MustCompile(createTablePattern).FindAllStringSubmatch(contentStr, -1)
		for _, match := range createMatches {
			if len(match) > 1 {
				tableName := match[1]
				// If a table is created after being dropped, it's no longer considered dropped
				if droppedTables[tableName] {
					delete(droppedTables, tableName)
					g.logger.Debugf("Table %s was recreated in %s after being dropped", tableName, fileName)
				}
			}
		}

		// Look for DROP TABLE statements
		tableMatches := regexp.MustCompile(dropTablePattern).FindAllStringSubmatch(contentStr, -1)
		for _, match := range tableMatches {
			if len(match) > 1 {
				tableName := match[1]
				droppedTables[tableName] = true
				g.logger.Debugf("Found existing DROP TABLE for %s in %s", tableName, fileName)

				// When a table is dropped, all its columns are implicitly dropped too
				// Remove any tracked dropped columns for this table
				for key := range droppedColumns {
					if strings.HasPrefix(key, tableName+".") {
						delete(droppedColumns, key)
					}
				}
			}
		}

		// Look for DROP COLUMN statements
		columnMatches := regexp.MustCompile(dropColumnPattern).FindAllStringSubmatch(contentStr, -1)
		for _, match := range columnMatches {
			if len(match) > 2 {
				tableName := match[1]
				columnName := match[2]

				// Only track dropped columns for tables that haven't been dropped
				if !droppedTables[tableName] {
					key := fmt.Sprintf("%s.%s", tableName, columnName)
					droppedColumns[key] = true
					g.logger.Debugf("Found existing DROP COLUMN for %s in %s", key, fileName)
				}
			}
		}
	}

	// Return the combined results
	result := make(map[string]bool)
	for k, v := range droppedTables {
		result[k] = v
	}
	for k, v := range droppedColumns {
		result[k] = v
	}

	return result, nil
}

// detectColumnRenames analyzes the changes to identify potential column renames
// Returns a map of renamed columns where the key is "tableName.newColumnName" and
// the value is the original column name
func (g *Generator) detectColumnRenames(changes *ChangeSet) map[string]string {
	// Map to store potential renames: key = "tableName.newColumnName", value = oldColumnName
	renamedColumns := make(map[string]string)

	// Safety check for nil changes
	if changes == nil || len(changes.Changes) == 0 {
		return renamedColumns
	}

	// Maps to track dropped and added columns by table
	droppedByTable := make(map[string][]*Change)
	addedByTable := make(map[string][]*Change)

	// First, categorize changes by table and type
	for _, change := range changes.Changes {
		if change == nil {
			g.logger.Warn("Skipping nil change in detectColumnRenames")
			continue
		}

		if change.Type == DropColumn && change.ParentName != "" {
			droppedByTable[change.ParentName] = append(droppedByTable[change.ParentName], change)
			g.logger.Debugf("Tracking dropped column %s.%s for potential rename", change.ParentName, change.ObjectName)
		} else if change.Type == AddColumn && change.ParentName != "" {
			addedByTable[change.ParentName] = append(addedByTable[change.ParentName], change)
			g.logger.Debugf("Tracking added column %s.%s for potential rename", change.ParentName, change.ObjectName)
		}
	}

	// For each table with both dropped and added columns, look for potential renames
	for tableName, droppedColumns := range droppedByTable {
		addedColumns, hasAdded := addedByTable[tableName]
		if !hasAdded || len(addedColumns) == 0 {
			continue // No added columns for this table
		}

		g.logger.Debugf("Table %s has both dropped and added columns, checking for renames", tableName)

		// Compare each dropped column with each added column
		for _, dropped := range droppedColumns {
			if dropped == nil || dropped.ObjectName == "" {
				continue
			}

			for _, added := range addedColumns {
				if added == nil || added.ObjectName == "" {
					continue
				}

				// Skip if the added column is already identified as a rename
				key := fmt.Sprintf("%s.%s", tableName, added.ObjectName)
				if _, exists := renamedColumns[key]; exists {
					continue
				}

				// Initialize metadata maps if they don't exist
				if dropped.Metadata == nil {
					dropped.Metadata = make(map[string]interface{})
				}
				if added.Metadata == nil {
					added.Metadata = make(map[string]interface{})
				}

				// Check if the column types are similar
				droppedType := ""
				addedType := ""

				if dropped.Object != nil && dropped.Object.Metadata != nil {
					if t, ok := dropped.Object.Metadata["type"].(string); ok {
						droppedType = strings.ToLower(t)
					}
				}

				if added.Object != nil && added.Object.Metadata != nil {
					if t, ok := added.Object.Metadata["type"].(string); ok {
						addedType = strings.ToLower(t)
					}
				}

				// If types match or are similar, consider it a potential rename
				if droppedType != "" && addedType != "" {
					typesMatch := droppedType == addedType ||
						strings.HasPrefix(droppedType, addedType) ||
						strings.HasPrefix(addedType, droppedType)

					if typesMatch {
						// Store the rename information
						renamedColumns[key] = dropped.ObjectName
						g.logger.Debugf("Detected column rename: %s.%s -> %s.%s",
							tableName, dropped.ObjectName, tableName, added.ObjectName)

						// Mark the changes as part of a rename operation
						dropped.Metadata["renamed_to"] = added.ObjectName
						added.Metadata["renamed_from"] = dropped.ObjectName

						// Ensure both changes are kept in the changeset
						dropped.Metadata["is_rename_part"] = true
						added.Metadata["is_rename_part"] = true

						// Store original column definition for down migration
						if dropped.Object != nil && dropped.Object.Metadata != nil {
							if def, ok := dropped.Object.Metadata["definition"].(string); ok {
								dropped.Metadata["original_definition"] = def
								g.logger.Debugf("Stored original definition for renamed column: %s", def)
							}
						}

						// Once we've found a match, break the inner loop
						break
					}
				}
			}
		}
	}

	return renamedColumns
}
