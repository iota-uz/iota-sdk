package diff

import (
	"sort"
	"strings"

	"github.com/auxten/postgresql-parser/pkg/sql/sem/tree"
	"github.com/iota-uz/iota-sdk/pkg/schema/common"
	"github.com/sirupsen/logrus"
)

// Initialize analyzer logger as an instance variable, not package variable
type analyzerLogger struct {
	logger *logrus.Logger
}

func newAnalyzerLogger() *analyzerLogger {
	return &analyzerLogger{
		logger: logrus.New(),
	}
}

// SetLogLevel sets the logging level for the analyzer
func (l *analyzerLogger) SetLogLevel(level logrus.Level) {
	l.logger.SetLevel(level)
}

// Analyzer handles schema comparison and change detection
type Analyzer struct {
	oldSchema *common.Schema
	newSchema *common.Schema
	options   AnalyzerOptions
	logger    *analyzerLogger
}

type AnalyzerOptions struct {
	IgnoreCase          bool
	IgnoreWhitespace    bool
	DetectRenames       bool
	ValidateConstraints bool
}

// Compare analyzes differences between two schemas
func (a *Analyzer) Compare() (*common.ChangeSet, error) {
	changes := NewChangeSet()
	logger := a.logger.logger

	// Find added and modified tables
	logger.Debugf("Processing tables from new schema")
	for tableName, newTable := range a.newSchema.Tables {
		tableNameLower := strings.ToLower(tableName)
		logger.WithFields(logrus.Fields{
			"table": tableName,
		}).Debug("Processing table from new schema")

		if oldTable, exists := a.oldSchema.Tables[tableNameLower]; !exists {
			// New table
			logger.WithFields(logrus.Fields{
				"table": tableName,
			}).Debug("Found new table")
			changes.Changes = append(changes.Changes, &common.Change{
				Type:       common.CreateTable,
				Object:     newTable,
				ObjectName: tableName,
				ParentName: "",
				Reversible: true,
			})
		} else {
			// Existing table - compare columns
			tableDiffs := a.compareTableColumns(tableNameLower, oldTable, newTable)
			for _, diff := range tableDiffs {
				changes.Changes = append(changes.Changes, diff)
			}
		}
	}

	// Find dropped tables
	for tableName := range a.oldSchema.Tables {
		tableNameLower := strings.ToLower(tableName)
		if _, exists := a.newSchema.Tables[tableNameLower]; !exists {
			logger.WithField("table", tableName).Debug("Found dropped table")
			changes.Changes = append(changes.Changes, &common.Change{
				Type:       common.DropTable,
				Object:     a.oldSchema.Tables[tableNameLower],
				ObjectName: tableName,
				ParentName: "",
				Reversible: true,
			})
		}
	}

	// Find added and modified indexes
	logger.Debugf("Processing indexes from new schema")
	for indexName, newIndex := range a.newSchema.Indexes {
		indexNameLower := strings.ToLower(indexName)
		tableName := newIndex.Table.String()
		
		logger.WithFields(logrus.Fields{
			"index": indexName,
			"table": tableName,
		}).Debug("Processing index from new schema")

		if oldIndex, exists := a.oldSchema.Indexes[indexNameLower]; !exists {
			// New index
			logger.WithFields(logrus.Fields{
				"index": indexName,
				"table": tableName,
			}).Debug("Found new index")
			changes.Changes = append(changes.Changes, &common.Change{
				Type:       common.AddIndex,
				Object:     newIndex,
				ObjectName: indexName,
				ParentName: tableName,
				Reversible: true,
			})
		} else {
			// Existing index - check if modified
			if !a.indexesEqual(oldIndex, newIndex) {
				logger.WithFields(logrus.Fields{
					"index": indexName,
					"table": tableName,
				}).Debug("Found modified index")
				changes.Changes = append(changes.Changes, &common.Change{
					Type:       common.ModifyIndex,
					Object:     newIndex,
					ObjectName: indexName,
					ParentName: tableName,
					Reversible: true,
					Metadata: map[string]interface{}{
						"old_definition": oldIndex.String(),
						"new_definition": newIndex.String(),
					},
				})
			}
		}
	}

	// Find dropped indexes
	for indexName, oldIndex := range a.oldSchema.Indexes {
		indexNameLower := strings.ToLower(indexName)
		if _, exists := a.newSchema.Indexes[indexNameLower]; !exists {
			tableName := oldIndex.Table.String()
			logger.WithField("index", indexName).Debug("Found dropped index")
			changes.Changes = append(changes.Changes, &common.Change{
				Type:       common.DropIndex,
				Object:     oldIndex,
				ObjectName: indexName,
				ParentName: tableName,
				Reversible: true,
			})
		}
	}

	logger.WithFields(logrus.Fields{
		"total_changes": len(changes.Changes),
		"tables":        len(a.newSchema.Tables),
		"indexes":       len(a.newSchema.Indexes),
	}).Info("Completed schema comparison")
	return changes, nil
}

// NewAnalyzer creates a new schema analyzer
func NewAnalyzer(oldSchema, newSchema *common.Schema, opts AnalyzerOptions) *Analyzer {
	return &Analyzer{
		oldSchema: oldSchema,
		newSchema: newSchema,
		options:   opts,
		logger:    newAnalyzerLogger(),
	}
}

// SetLogLevel sets the logging level for the analyzer
func (a *Analyzer) SetLogLevel(level logrus.Level) {
	a.logger.SetLogLevel(level)
}

// compareTableColumns compares columns between old and new tables
func (a *Analyzer) compareTableColumns(tableName string, oldTable, newTable *tree.CreateTable) []*common.Change {
	var changes []*common.Change
	logger := a.logger.logger
	
	// Get columns from old table
	oldColumns := make(map[string]*tree.ColumnTableDef)
	for _, def := range oldTable.Defs {
		if colDef, ok := def.(*tree.ColumnTableDef); ok {
			colName := strings.ToLower(string(colDef.Name))
			oldColumns[colName] = colDef
			logger.WithFields(logrus.Fields{
				"table":  tableName,
				"column": colName,
				"type":   colDef.Type.String(),
			}).Debug("Loaded column from old schema")
		}
	}
	
	// Get columns from new table and compare
	newColumns := make(map[string]*tree.ColumnTableDef)
	for _, def := range newTable.Defs {
		if colDef, ok := def.(*tree.ColumnTableDef); ok {
			colName := strings.ToLower(string(colDef.Name))
			newColumns[colName] = colDef
			
			logger.WithFields(logrus.Fields{
				"table":  tableName,
				"column": colName,
				"type":   colDef.Type.String(),
			}).Debug("Processing column from new schema")
			
			// Check if column exists in old table
			if oldCol, exists := oldColumns[colName]; exists {
				// Compare column definitions
				if !a.columnsEqual(oldCol, colDef) {
					logger.WithFields(logrus.Fields{
						"table":     tableName,
						"column":    colName,
						"old_type":  oldCol.Type.String(),
						"new_type":  colDef.Type.String(),
					}).Debug("Found modified column")
					
					changes = append(changes, &common.Change{
						Type:       common.ModifyColumn,
						Object:     colDef,
						ObjectName: string(colDef.Name),
						ParentName: tableName,
						Reversible: true,
						Metadata: map[string]interface{}{
							"old_definition": oldCol.String(),
							"new_definition": colDef.String(),
							"old_type":       oldCol.Type.String(),
							"new_type":       colDef.Type.String(),
						},
					})
				}
			} else {
				// New column
				logger.WithFields(logrus.Fields{
					"table":  tableName,
					"column": colName,
				}).Debug("Found new column")
				
				changes = append(changes, &common.Change{
					Type:       common.AddColumn,
					Object:     colDef,
					ObjectName: string(colDef.Name),
					ParentName: tableName,
					Reversible: true,
				})
			}
		}
	}
	
	// Check for dropped columns
	for colName, oldCol := range oldColumns {
		if _, exists := newColumns[colName]; !exists {
			logger.WithFields(logrus.Fields{
				"table":  tableName,
				"column": colName,
			}).Debug("Found dropped column")
			
			changes = append(changes, &common.Change{
				Type:       common.DropColumn,
				Object:     oldCol,
				ObjectName: string(oldCol.Name),
				ParentName: tableName,
				Reversible: true,
			})
		}
	}
	
	return changes
}

// columnsEqual compares two column definitions
func (a *Analyzer) columnsEqual(oldCol, newCol *tree.ColumnTableDef) bool {
	logger := a.logger.logger
	
	if oldCol == nil || newCol == nil {
		logger.Debug("One of the columns is nil")
		return false
	}

	// Compare column types
	oldType := oldCol.Type.String()
	newType := newCol.Type.String()
	
	logger.WithFields(logrus.Fields{
		"column":    string(oldCol.Name),
		"old_type":  oldType,
		"new_type":  newType,
	}).Debug("Comparing column types")
	
	if oldType != newType {
		logger.WithFields(logrus.Fields{
			"column":   string(oldCol.Name),
			"old_type": oldType,
			"new_type": newType,
		}).Debug("Column type mismatch")
		return false
	}
	
	// Compare nullability
	if oldCol.Nullable.Nullability != newCol.Nullable.Nullability {
		logger.WithFields(logrus.Fields{
			"column":        string(oldCol.Name),
			"old_nullable":  oldCol.Nullable.Nullability,
			"new_nullable":  newCol.Nullable.Nullability,
		}).Debug("Column nullability mismatch")
		return false
	}
	
	// Compare default expressions
	oldHasDefault := oldCol.DefaultExpr.Expr != nil
	newHasDefault := newCol.DefaultExpr.Expr != nil
	
	if oldHasDefault != newHasDefault {
		logger.WithFields(logrus.Fields{
			"column":       string(oldCol.Name),
			"old_default":  oldHasDefault,
			"new_default":  newHasDefault,
		}).Debug("Column default presence mismatch")
		return false
	}
	
	if oldHasDefault && newHasDefault {
		oldDefault := oldCol.DefaultExpr.Expr.String()
		newDefault := newCol.DefaultExpr.Expr.String()
		if oldDefault != newDefault {
			logger.WithFields(logrus.Fields{
				"column":       string(oldCol.Name),
				"old_default":  oldDefault,
				"new_default":  newDefault,
			}).Debug("Column default value mismatch")
			return false
		}
	}
	
	// Compare primary key flag
	if oldCol.PrimaryKey.IsPrimaryKey != newCol.PrimaryKey.IsPrimaryKey {
		logger.WithFields(logrus.Fields{
			"column":      string(oldCol.Name),
			"old_pk":      oldCol.PrimaryKey.IsPrimaryKey,
			"new_pk":      newCol.PrimaryKey.IsPrimaryKey,
		}).Debug("Column primary key flag mismatch")
		return false
	}
	
	// Compare uniqueness
	if oldCol.Unique != newCol.Unique {
		logger.WithFields(logrus.Fields{
			"column":      string(oldCol.Name),
			"old_unique":  oldCol.Unique,
			"new_unique":  newCol.Unique,
		}).Debug("Column uniqueness mismatch")
		return false
	}
	
	// Compare references
	oldHasRef := oldCol.References.Table != nil
	newHasRef := newCol.References.Table != nil
	
	if oldHasRef != newHasRef {
		logger.WithFields(logrus.Fields{
			"column":    string(oldCol.Name),
			"old_ref":   oldHasRef,
			"new_ref":   newHasRef,
		}).Debug("Column references presence mismatch")
		return false
	}
	
	if oldHasRef && newHasRef {
		oldRef := oldCol.References.Table.String()
		newRef := newCol.References.Table.String()
		oldRefCol := oldCol.References.Col.String()
		newRefCol := newCol.References.Col.String()
		
		if oldRef != newRef || oldRefCol != newRefCol {
			logger.WithFields(logrus.Fields{
				"column":      string(oldCol.Name),
				"old_ref":     oldRef,
				"old_ref_col": oldRefCol,
				"new_ref":     newRef, 
				"new_ref_col": newRefCol,
			}).Debug("Column reference mismatch")
			return false
		}
	}

	logger.WithFields(logrus.Fields{
		"column": string(oldCol.Name),
		"type":   oldType,
	}).Debug("Column definitions are equal")
	return true
}

// indexesEqual compares two index definitions
func (a *Analyzer) indexesEqual(oldIndex, newIndex *tree.CreateIndex) bool {
	if oldIndex == nil || newIndex == nil {
		return false
	}

	// Compare table names
	oldTable := strings.ToLower(oldIndex.Table.String())
	newTable := strings.ToLower(newIndex.Table.String())
	if oldTable != newTable {
		return false
	}

	// Compare uniqueness
	if oldIndex.Unique != newIndex.Unique {
		return false
	}

	// Compare columns
	if len(oldIndex.Columns) != len(newIndex.Columns) {
		return false
	}
	
	// Extract column names for comparison
	oldCols := make([]string, len(oldIndex.Columns))
	newCols := make([]string, len(newIndex.Columns))
	
	for i, col := range oldIndex.Columns {
		oldCols[i] = strings.ToLower(string(col.Column))
	}
	
	for i, col := range newIndex.Columns {
		newCols[i] = strings.ToLower(string(col.Column))
	}
	
	// Sort for consistent comparison
	sort.Strings(oldCols)
	sort.Strings(newCols)
	
	// Compare sorted column lists
	for i := range oldCols {
		if oldCols[i] != newCols[i] {
			return false
		}
	}
	
	return true
}
