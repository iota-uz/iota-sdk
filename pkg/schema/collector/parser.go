package collector

import (
	"fmt"
	"strings"

	"github.com/auxten/postgresql-parser/pkg/sql/parser"
	pgtree "github.com/auxten/postgresql-parser/pkg/sql/sem/tree"
	"github.com/auxten/postgresql-parser/pkg/walk"
	"github.com/sirupsen/logrus"
)

type PostgresParser struct {
	logger *logrus.Logger
}

func NewPostgresParser(logger *logrus.Logger) *PostgresParser {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.InfoLevel)
	}

	return &PostgresParser{
		logger: logger,
	}
}

func (p *PostgresParser) GetDialect() string {
	return "postgres"
}

func (p *PostgresParser) ParseSQL(sql string) (*SchemaTree, error) {
	tree := NewSchemaTree()

	stmts, err := parser.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SQL: %w", err)
	}

	state := &parserState{
		tableNodes: make(map[string]*Node),
		indexNodes: make(map[string]*Node),
		logger:     p.logger,
	}

	walker := &walk.AstWalker{
		Fn: state.processNode,
	}

	_, _ = walker.Walk(stmts, nil)

	// Build final tree
	tree.Root.Children = state.buildFinalNodes()

	return tree, nil
}

type parserState struct {
	tableNodes   map[string]*Node
	indexNodes   map[string]*Node
	currentTable *Node
	logger       *logrus.Logger
}

func (s *parserState) processNode(ctx interface{}, node interface{}) bool {
	switch n := node.(type) {
	case *pgtree.CreateTable:
		s.handleCreateTable(n)
	case *pgtree.ColumnTableDef:
		s.handleColumnDef(n)
	case *pgtree.CreateIndex:
		s.handleCreateIndex(n)
	case *pgtree.AlterTable:
		s.handleAlterTable(n)
	case *pgtree.DropTable:
		s.handleDropTable(n)
	}
	return false
}

func (s *parserState) handleCreateTable(n *pgtree.CreateTable) {
	if n == nil {
		return
	}

	tableName := strings.ToLower(n.Table.Table())
	s.logger.Debugf("Processing CREATE TABLE: %s", tableName)

	tableNode := &Node{
		Type:     NodeTable,
		Name:     tableName,
		Children: make([]*Node, 0),
		Metadata: make(map[string]interface{}),
	}

	s.tableNodes[tableName] = tableNode
	s.currentTable = tableNode
}

func (s *parserState) handleColumnDef(n *pgtree.ColumnTableDef) {
	if s.currentTable == nil || n == nil {
		return
	}

	columnName := strings.ToLower(string(n.Name))
	s.logger.Debugf("Processing column: %s", columnName)

	columnNode := &Node{
		Type:     NodeColumn,
		Name:     columnName,
		Metadata: make(map[string]interface{}),
	}

	// Extract column type
	columnType := ""
	if n.Type != nil {
		columnType = n.Type.SQLString()
	}
	columnNode.Metadata["type"] = columnType
	columnNode.Metadata["fullType"] = columnType

	// Process constraints
	constraints := s.extractColumnConstraints(n)
	if constraints != "" {
		columnNode.Metadata["constraints"] = constraints
		columnNode.Metadata["definition"] = fmt.Sprintf("%s %s %s",
			columnName, columnType, constraints)
	} else {
		columnNode.Metadata["definition"] = fmt.Sprintf("%s %s",
			columnName, columnType)
	}

	s.currentTable.Children = append(s.currentTable.Children, columnNode)
}

func (s *parserState) handleCreateIndex(n *pgtree.CreateIndex) {
	if n == nil {
		return
	}

	indexName := strings.ToLower(string(n.Name.String()))
	tableName := strings.ToLower(n.Table.Table())
	s.logger.Debugf("Processing CREATE INDEX: %s on table %s", indexName, tableName)

	columns := make([]string, 0)
	for _, col := range n.Columns {
		columns = append(columns, strings.ToLower(string(col.Column)))
	}

	indexNode := &Node{
		Type: NodeIndex,
		Name: indexName,
		Metadata: map[string]interface{}{
			"table":     tableName,
			"columns":   strings.Join(columns, ", "),
			"is_unique": n.Unique,
		},
	}

	s.indexNodes[indexName] = indexNode
}

func (s *parserState) handleAlterTable(n *pgtree.AlterTable) {
	if n == nil || n.Table == nil {
		return
	}

	tableName := strings.ToLower(n.Table.String())
	s.logger.Debugf("Processing ALTER TABLE: %s", tableName)

	// Find or create table node
	tableNode, exists := s.tableNodes[tableName]
	if !exists {
		tableNode = &Node{
			Type:     NodeTable,
			Name:     tableName,
			Children: make([]*Node, 0),
			Metadata: make(map[string]interface{}),
		}
		s.tableNodes[tableName] = tableNode
	}
	s.currentTable = tableNode

	for _, cmd := range n.Cmds {
		switch altCmd := cmd.(type) {
		case *pgtree.AlterTableAddColumn:
			s.handleAddColumn(altCmd)
		case *pgtree.AlterTableAlterColumnType:
			s.handleAlterColumnType(altCmd)
		case *pgtree.AlterTableDropColumn:
			s.handleDropColumn(altCmd)
		}
	}
}

func (s *parserState) handleAddColumn(cmd *pgtree.AlterTableAddColumn) {
	if cmd.ColumnDef == nil || s.currentTable == nil {
		return
	}

	columnName := strings.ToLower(string(cmd.ColumnDef.Name))
	s.logger.Debugf("Processing ADD COLUMN: %s", columnName)

	columnType := ""
	if cmd.ColumnDef.Type != nil {
		columnType = cmd.ColumnDef.Type.SQLString()
	}

	columnNode := &Node{
		Type: NodeColumn,
		Name: columnName,
		Metadata: map[string]interface{}{
			"type":     columnType,
			"fullType": columnType,
		},
	}

	constraints := s.extractColumnConstraints(cmd.ColumnDef)
	if constraints != "" {
		columnNode.Metadata["constraints"] = constraints
		columnNode.Metadata["definition"] = fmt.Sprintf("%s %s %s",
			columnName, columnType, constraints)
	} else {
		columnNode.Metadata["definition"] = fmt.Sprintf("%s %s",
			columnName, columnType)
	}

	s.currentTable.Children = append(s.currentTable.Children, columnNode)
}

func (s *parserState) handleAlterColumnType(cmd *pgtree.AlterTableAlterColumnType) {
	if s.currentTable == nil {
		return
	}

	columnName := strings.ToLower(string(cmd.Column))
	newType := cmd.ToType.SQLString()
	s.logger.Debugf("Processing ALTER COLUMN TYPE: %s to %s", columnName, newType)

	for _, child := range s.currentTable.Children {
		if child.Type == NodeColumn && strings.EqualFold(child.Name, columnName) {
			child.Metadata["type"] = newType
			child.Metadata["fullType"] = newType

			if constraints, ok := child.Metadata["constraints"].(string); ok && constraints != "" {
				child.Metadata["definition"] = fmt.Sprintf("%s %s %s",
					columnName, newType, constraints)
			} else {
				child.Metadata["definition"] = fmt.Sprintf("%s %s",
					columnName, newType)
			}
			break
		}
	}
}

func (s *parserState) handleDropColumn(cmd *pgtree.AlterTableDropColumn) {
	if s.currentTable == nil {
		return
	}

	columnName := strings.ToLower(string(cmd.Column))
	s.logger.Debugf("Processing DROP COLUMN: %s", columnName)

	children := make([]*Node, 0)
	for _, child := range s.currentTable.Children {
		if child.Type != NodeColumn || !strings.EqualFold(child.Name, columnName) {
			children = append(children, child)
		}
	}
	s.currentTable.Children = children
}

func (s *parserState) handleDropTable(n *pgtree.DropTable) {
	if n == nil || n.Names == nil {
		return
	}

	for _, name := range n.Names {
		tableName := strings.ToLower(name.Table())
		s.logger.Debugf("Processing DROP TABLE: %s", tableName)
		delete(s.tableNodes, tableName)
	}
}

func (s *parserState) extractColumnConstraints(n *pgtree.ColumnTableDef) string {
	constraints := make([]string, 0)

	if n.Nullable.Nullability == pgtree.NotNull {
		constraints = append(constraints, "NOT NULL")
	}

	if n.PrimaryKey.IsPrimaryKey {
		constraints = append(constraints, "PRIMARY KEY")
	}

	if n.Unique {
		constraints = append(constraints, "UNIQUE")
	}

	if n.DefaultExpr.Expr != nil {
		constraints = append(constraints, fmt.Sprintf("DEFAULT %s", n.DefaultExpr.Expr.String()))
	}

	if n.References.Table != nil {
		constraints = append(constraints, fmt.Sprintf("REFERENCES %s(%s)",
			n.References.Table.String(),
			n.References.Col.String()))
	}

	return strings.Join(constraints, " ")
}

func (s *parserState) buildFinalNodes() []*Node {
	nodes := make([]*Node, 0)

	// Add tables
	for _, tableNode := range s.tableNodes {
		nodes = append(nodes, tableNode)
	}

	// Add indexes
	for _, indexNode := range s.indexNodes {
		nodes = append(nodes, indexNode)
	}

	return nodes
}
