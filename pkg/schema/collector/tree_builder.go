package collector

import (
	"strings"
)

type schemaState struct {
	tables  map[string]map[string]*columnState // table -> column -> state
	indexes map[string]*indexState
	drops   map[string]bool
}

type columnState struct {
	node      *Node
	timestamp int64
	type_     string
	lastFile  string
}

type indexState struct {
	node      *Node
	timestamp int64
	lastFile  string
}

func newSchemaState() *schemaState {
	return &schemaState{
		tables:  make(map[string]map[string]*columnState),
		indexes: make(map[string]*indexState),
		drops:   make(map[string]bool),
	}
}

func (s *schemaState) updateFromParsedTree(tree *SchemaTree, timestamp int64, fileName string) {
	for _, node := range tree.Root.Children {
		switch node.Type {
		case NodeTable:
			s.updateTableState(node, timestamp, fileName)
		case NodeIndex:
			s.updateIndexState(node, timestamp, fileName)
		}
	}
}

func (s *schemaState) updateTableState(node *Node, timestamp int64, fileName string) {
	tableName := strings.ToLower(node.Name)

	// Handle dropped tables
	if s.drops[tableName] {
		return
	}

	if _, exists := s.tables[tableName]; !exists {
		s.tables[tableName] = make(map[string]*columnState)
	}

	for _, col := range node.Children {
		if col.Type == NodeColumn {
			s.updateColumnState(tableName, col, timestamp, fileName)
		}
	}
}

func (s *schemaState) updateColumnState(tableName string, col *Node, timestamp int64, fileName string) {
	colName := strings.ToLower(col.Name)
	currentState := s.tables[tableName][colName]

	newType := s.extractColumnType(col)

	if shouldUpdateColumn(currentState, timestamp, newType) {
		s.tables[tableName][colName] = &columnState{
			node:      cleanNode(col),
			timestamp: timestamp,
			type_:     newType,
			lastFile:  fileName,
		}
	}
}

func (s *schemaState) updateIndexState(node *Node, timestamp int64, fileName string) {
	indexName := strings.ToLower(node.Name)
	currentState := s.indexes[indexName]

	if shouldUpdateIndex(currentState, timestamp) {
		s.indexes[indexName] = &indexState{
			node:      cleanNode(node),
			timestamp: timestamp,
			lastFile:  fileName,
		}
	}
}

func (s *schemaState) buildFinalTree() *SchemaTree {
	tree := NewSchemaTree()

	// Add tables
	for tableName, columns := range s.tables {
		if s.drops[tableName] {
			continue
		}

		tableNode := &Node{
			Type:     NodeTable,
			Name:     tableName,
			Children: make([]*Node, 0, len(columns)),
			Metadata: make(map[string]interface{}),
		}

		for _, state := range columns {
			tableNode.Children = append(tableNode.Children, state.node)
		}

		tree.Root.Children = append(tree.Root.Children, tableNode)
	}

	// Add indexes
	for _, state := range s.indexes {
		tree.Root.Children = append(tree.Root.Children, state.node)
	}

	return tree
}

// Helper functions

func shouldUpdateColumn(current *columnState, newTimestamp int64, newType string) bool {
	return current == nil || (newTimestamp > current.timestamp && newType != current.type_)
}

func shouldUpdateIndex(current *indexState, newTimestamp int64) bool {
	return current == nil || newTimestamp > current.timestamp
}

func cleanNode(node *Node) *Node {
	cleaned := &Node{
		Type:     node.Type,
		Name:     node.Name,
		Children: make([]*Node, len(node.Children)),
		Metadata: make(map[string]interface{}),
	}

	// Clean metadata
	for k, v := range node.Metadata {
		if strVal, ok := v.(string); ok {
			cleaned.Metadata[k] = strings.TrimRight(strVal, ";")
		} else {
			cleaned.Metadata[k] = v
		}
	}

	// Clean children recursively
	for i, child := range node.Children {
		cleaned.Children[i] = cleanNode(child)
	}

	return cleaned
}

func (s *schemaState) extractColumnType(node *Node) string {
	if fullType, ok := node.Metadata["fullType"].(string); ok {
		return strings.ToLower(strings.TrimRight(fullType, ";"))
	}
	if typeStr, ok := node.Metadata["type"].(string); ok {
		return strings.ToLower(strings.TrimRight(typeStr, ";"))
	}
	return ""
}
