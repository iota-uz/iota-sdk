package diff

import (
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/schema/types"
	"github.com/sirupsen/logrus"
)

var logger = logrus.New()

// SetLogLevel sets the logging level for the analyzer
func SetLogLevel(level logrus.Level) {
	logger.SetLevel(level)
}

// func init() {
// 	// logger.SetLevel(logrus.InfoLevel) // Default to INFO level

// 	// Test log to verify logger is working
// 	// logger.Debug("Schema analyzer logger initialized")
// }

// Analyzer handles schema comparison and change detection
type Analyzer struct {
	oldSchema *types.SchemaTree
	newSchema *types.SchemaTree
	options   AnalyzerOptions
}

type AnalyzerOptions struct {
	IgnoreCase          bool
	IgnoreWhitespace    bool
	DetectRenames       bool
	ValidateConstraints bool
}

// Compare analyzes differences between two schema trees
func (a *Analyzer) Compare() (*ChangeSet, error) {
	changes := NewChangeSet()

	// Create maps for quick lookup
	oldTables := make(map[string]*types.Node)
	oldIndexes := make(map[string]*types.Node)
	newTables := make(map[string]*types.Node)
	newIndexes := make(map[string]*types.Node)

	// Map old schema objects
	for _, node := range a.oldSchema.Root.Children {
		switch node.Type {
		case types.NodeTable:
			tableName := strings.ToLower(node.Name)
			oldTables[tableName] = node
			logger.WithFields(logrus.Fields{
				"table":   node.Name,
				"columns": len(node.Children),
			}).Debug("Loaded table from old schema")
		case types.NodeIndex:
			indexName := strings.ToLower(node.Name)
			oldIndexes[indexName] = node
			logger.WithFields(logrus.Fields{
				"index":  node.Name,
				"table":  node.Metadata["table"],
				"unique": node.Metadata["is_unique"],
			}).Debug("Loaded index from old schema")
		}
	}

	// Find added and modified tables and indexes
	logger.Debugf("Processing %d nodes from new schema", len(a.newSchema.Root.Children))
	for _, node := range a.newSchema.Root.Children {
		switch node.Type {
		case types.NodeTable:
			tableName := strings.ToLower(node.Name)
			newTables[tableName] = node
			logger.WithFields(logrus.Fields{
				"table":   node.Name,
				"columns": len(node.Children),
			}).Debug("Processing table from new schema")

			oldTable, exists := oldTables[tableName]
			if !exists {
				logger.WithFields(logrus.Fields{
					"table": node.Name,
				}).Debug("Found new table")
				changes.Changes = append(changes.Changes, &Change{
					Type:       CreateTable,
					Object:     node,
					ObjectName: node.Name,
					ParentName: "",
					Reversible: true,
				})
			} else {
				logger.WithFields(logrus.Fields{
					"table":       node.Name,
					"old_columns": len(oldTable.Children),
					"new_columns": len(node.Children),
				}).Debug("Comparing existing table")

				tableDiffs := a.compareTable(oldTable, node)
				for _, diff := range tableDiffs {
					if diff.Type == ModifyColumn || diff.Type == AddColumn || diff.Type == DropColumn {
						diff.ParentName = node.Name
						diff.ObjectName = diff.Object.Name
					} else {
						diff.ObjectName = node.Name
					}
					changes.Changes = append(changes.Changes, diff)
				}
			}
		case types.NodeIndex:
			indexName := strings.ToLower(node.Name)
			newIndexes[indexName] = node
			logger.WithFields(logrus.Fields{
				"index":  node.Name,
				"table":  node.Metadata["table"],
				"unique": node.Metadata["is_unique"],
			}).Debug("Processing index from new schema")

			oldIndex, exists := oldIndexes[indexName]
			if !exists {
				logger.WithFields(logrus.Fields{
					"index": node.Name,
					"table": node.Metadata["table"],
				}).Debug("Found new index")
				changes.Changes = append(changes.Changes, &Change{
					Type:       AddIndex,
					Object:     node,
					ObjectName: node.Name,
					ParentName: node.Metadata["table"].(string),
					Reversible: true,
				})
			} else {
				if !a.indexesEqual(oldIndex, node) {
					logger.WithFields(logrus.Fields{
						"index": node.Name,
						"table": node.Metadata["table"],
					}).Debug("Found modified index")
					changes.Changes = append(changes.Changes, &Change{
						Type:       ModifyIndex,
						Object:     node,
						ObjectName: node.Name,
						ParentName: node.Metadata["table"].(string),
						Reversible: true,
						Metadata: map[string]interface{}{
							"old_definition": oldIndex.Metadata["original_sql"],
							"new_definition": node.Metadata["original_sql"],
						},
					})
				}
			}
		default:
			logger.WithFields(logrus.Fields{
				"type": node.Type,
				"name": node.Name,
			}).Debug("Found unknown node type")
		}
	}

	// Find dropped tables and indexes
	for name, node := range oldTables {
		if _, exists := newTables[strings.ToLower(name)]; !exists {
			logger.WithField("table", name).Debug("Found dropped table")
			changes.Changes = append(changes.Changes, &Change{
				Type:       DropTable,
				Object:     node,
				ObjectName: name,
				ParentName: "",
				Reversible: true,
			})
		}
	}

	for name, node := range oldIndexes {
		if _, exists := newIndexes[strings.ToLower(name)]; !exists {
			logger.WithField("index", name).Debug("Found dropped index")
			changes.Changes = append(changes.Changes, &Change{
				Type:       DropIndex,
				Object:     node,
				ObjectName: name,
				ParentName: node.Metadata["table"].(string),
				Reversible: true,
			})
		}
	}

	logger.WithFields(logrus.Fields{
		"total_changes": len(changes.Changes),
		"tables":        len(newTables),
		"indexes":       len(newIndexes),
	}).Info("Completed schema comparison")
	return changes, nil
}

// NewAnalyzer creates a new schema analyzer
func NewAnalyzer(oldSchema, newSchema *types.SchemaTree, opts AnalyzerOptions) *Analyzer {
	return &Analyzer{
		oldSchema: oldSchema,
		newSchema: newSchema,
		options:   opts,
	}
}

func (a *Analyzer) compareTable(oldTable, newTable *types.Node) []*Change {
	var changes []*Change
	oldCols := make(map[string]*types.Node)
	newCols := make(map[string]*types.Node)

	logger.WithFields(logrus.Fields{
		"table":       newTable.Name,
		"old_columns": len(oldTable.Children),
		"new_columns": len(newTable.Children),
	}).Debug("Starting table comparison")

	// Map old columns
	for _, child := range oldTable.Children {
		if child.Type == types.NodeColumn {
			oldCols[strings.ToLower(child.Name)] = child
			logger.WithFields(logrus.Fields{
				"table":       oldTable.Name,
				"column":      child.Name,
				"type":        child.Metadata["type"],
				"constraints": child.Metadata["constraints"],
			}).Debug("Loaded column from old schema")
		}
	}

	// Compare new columns
	for _, child := range newTable.Children {
		if child.Type == types.NodeColumn {
			newCols[strings.ToLower(child.Name)] = child
			colName := strings.ToLower(child.Name)

			logger.WithFields(logrus.Fields{
				"table":       newTable.Name,
				"column":      child.Name,
				"type":        child.Metadata["type"],
				"constraints": child.Metadata["constraints"],
			}).Debug("Processing column from new schema")

			if oldCol, exists := oldCols[colName]; exists {
				logger.WithFields(logrus.Fields{
					"table":    newTable.Name,
					"column":   child.Name,
					"old_type": oldCol.Metadata["type"],
					"new_type": child.Metadata["type"],
				}).Debug("Comparing existing column")

				if !a.columnsEqual(oldCol, child) {
					logger.WithFields(logrus.Fields{
						"table":           newTable.Name,
						"column":          child.Name,
						"old_type":        oldCol.Metadata["type"],
						"new_type":        child.Metadata["type"],
						"old_constraints": oldCol.Metadata["constraints"],
						"new_constraints": child.Metadata["constraints"],
					}).Debug("Found modified column")

					changes = append(changes, &Change{
						Type:       ModifyColumn,
						Object:     child,
						ObjectName: child.Name,
						ParentName: newTable.Name,
						Reversible: true,
						Metadata: map[string]interface{}{
							"old_definition":  oldCol.Metadata["definition"],
							"new_definition":  child.Metadata["definition"],
							"old_type":        oldCol.Metadata["type"],
							"new_type":        child.Metadata["type"],
							"old_constraints": oldCol.Metadata["constraints"],
							"new_constraints": child.Metadata["constraints"],
						},
					})
				}
			} else {
				// New column
				logger.WithField("table", newTable.Name).Debug("Found new column")
				changes = append(changes, &Change{
					Type:       AddColumn,
					Object:     child,
					ObjectName: child.Name,
					ParentName: newTable.Name,
					Reversible: true,
				})
			}
		}
	}

	// Check for dropped columns
	for colName, oldCol := range oldCols {
		if _, exists := newCols[colName]; !exists {
			logger.WithField("table", newTable.Name).Debug("Found dropped column")
			changes = append(changes, &Change{
				Type:       DropColumn,
				Object:     oldCol,
				ObjectName: oldCol.Name,
				ParentName: newTable.Name,
				Reversible: true,
			})
		}
	}

	return changes
}

func (a *Analyzer) columnsEqual(oldCol, newCol *types.Node) bool {
	if oldCol == nil || newCol == nil {
		logger.Debug("One of the columns is nil")
		return false
	}

	// Get and normalize types
	oldType := strings.ToLower(oldCol.Metadata["type"].(string))
	newType := strings.ToLower(newCol.Metadata["type"].(string))

	// Log the raw types before any processing
	logger.WithFields(logrus.Fields{
		"column":          oldCol.Name,
		"old_type_raw":    oldType,
		"new_type_raw":    newType,
		"old_definition":  oldCol.Metadata["definition"],
		"new_definition":  newCol.Metadata["definition"],
		"old_constraints": oldCol.Metadata["constraints"],
		"new_constraints": newCol.Metadata["constraints"],
		"old_full_type":   oldCol.Metadata["fullType"],
		"new_full_type":   newCol.Metadata["fullType"],
	}).Debug("Starting column comparison")

	// Compare the full type definitions first
	oldFullType := strings.ToLower(oldCol.Metadata["fullType"].(string))
	newFullType := strings.ToLower(newCol.Metadata["fullType"].(string))

	if oldFullType != newFullType {
		logger.WithFields(logrus.Fields{
			"column":        oldCol.Name,
			"old_full_type": oldFullType,
			"new_full_type": newFullType,
		}).Debug("Full type definitions differ")
		return false
	}

	// Compare base types (varchar vs varchar)
	oldBaseType := strings.Split(oldType, "(")[0]
	newBaseType := strings.Split(newType, "(")[0]

	if oldBaseType != newBaseType {
		logger.WithFields(logrus.Fields{
			"column":   oldCol.Name,
			"old_type": oldType,
			"new_type": newType,
			"old_base": oldBaseType,
			"new_base": newBaseType,
		}).Debug("Base type mismatch")
		return false
	}

	// For VARCHAR types, compare lengths exactly as specified
	if oldBaseType == "varchar" {
		oldLen := ""
		newLen := ""

		if strings.Contains(oldFullType, "(") {
			oldLen = strings.Trim(strings.Split(oldFullType, "(")[1], ")")
		}
		if strings.Contains(newFullType, "(") {
			newLen = strings.Trim(strings.Split(newFullType, "(")[1], ")")
		}

		// If one has a length and the other doesn't, they're different
		if (oldLen == "" && newLen != "") || (oldLen != "" && newLen == "") {
			logger.WithFields(logrus.Fields{
				"column":   oldCol.Name,
				"old_type": oldFullType,
				"new_type": newFullType,
				"old_len":  oldLen,
				"new_len":  newLen,
			}).Debug("VARCHAR length specification mismatch")
			return false
		}

		// If both have lengths, compare them
		if oldLen != "" && newLen != "" && oldLen != newLen {
			logger.WithFields(logrus.Fields{
				"column":   oldCol.Name,
				"old_type": oldType,
				"new_type": newType,
				"old_len":  oldLen,
				"new_len":  newLen,
			}).Debug("VARCHAR length mismatch")
			return false
		}
	}

	// Compare constraints
	oldConstraints := strings.ToLower(strings.TrimSpace(oldCol.Metadata["constraints"].(string)))
	newConstraints := strings.ToLower(strings.TrimSpace(newCol.Metadata["constraints"].(string)))

	// Normalize constraint strings
	oldConstraints = normalizeConstraints(oldConstraints)
	newConstraints = normalizeConstraints(newConstraints)

	if oldConstraints != newConstraints {
		logger.WithFields(logrus.Fields{
			"column":          oldCol.Name,
			"old_constraints": oldConstraints,
			"new_constraints": newConstraints,
		}).Debug("Constraint mismatch")
		return false
	}

	logger.WithFields(logrus.Fields{
		"column":      oldCol.Name,
		"type":        oldType,
		"full_type":   oldFullType,
		"constraints": oldConstraints,
	}).Debug("Column definitions are equal")
	return true
}

// normalizeConstraints normalizes constraint strings for comparison
func normalizeConstraints(constraints string) string {
	// Split constraints into parts
	parts := strings.Fields(constraints)

	// Sort parts to ensure consistent ordering
	sort.Strings(parts)

	// Join back together
	return strings.Join(parts, " ")
}

func (a *Analyzer) indexesEqual(oldIndex, newIndex *types.Node) bool {
	if oldIndex == nil || newIndex == nil {
		return false
	}

	// Compare table names
	oldTable := strings.ToLower(oldIndex.Metadata["table"].(string))
	newTable := strings.ToLower(newIndex.Metadata["table"].(string))
	if oldTable != newTable {
		return false
	}

	// Compare uniqueness
	oldUnique := oldIndex.Metadata["is_unique"].(bool)
	newUnique := newIndex.Metadata["is_unique"].(bool)
	if oldUnique != newUnique {
		return false
	}

	// Compare columns (normalize and compare)
	oldCols := strings.ToLower(strings.ReplaceAll(oldIndex.Metadata["columns"].(string), " ", ""))
	newCols := strings.ToLower(strings.ReplaceAll(newIndex.Metadata["columns"].(string), " ", ""))
	return oldCols == newCols
}
