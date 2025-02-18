package ast

import (
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/schema/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestSetLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		level logrus.Level
	}{
		{
			name:  "Set debug level",
			level: logrus.DebugLevel,
		},
		{
			name:  "Set info level",
			level: logrus.InfoLevel,
		},
		{
			name:  "Set error level",
			level: logrus.ErrorLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLogLevel(tt.level)
			assert.Equal(t, tt.level, logger.GetLevel())
		})
	}
}

func TestParseCreateTable(t *testing.T) {
	p := NewParser("postgres", ParserOptions{})

	tests := []struct {
		name          string
		sql           string
		expectedTable string
		expectedCols  []string
		expectedError bool
	}{
		{
			name: "Simple table with basic columns",
			sql: `CREATE TABLE users (
				id SERIAL PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				email TEXT UNIQUE
			);`,
			expectedTable: "users",
			expectedCols:  []string{"id", "name", "email"},
			expectedError: false,
		},
		{
			name: "Table with quoted identifiers",
			sql: `CREATE TABLE "user_data" (
				"user_id" INTEGER,
				"full_name" VARCHAR(100)
			);`,
			expectedTable: "user_data",
			expectedCols:  []string{"user_id", "full_name"},
			expectedError: false,
		},
		{
			name: "Table with constraints",
			sql: `CREATE TABLE products (
				id SERIAL,
				name TEXT NOT NULL,
				price DECIMAL(10,2),
				CONSTRAINT pk_products PRIMARY KEY (id),
				CONSTRAINT uq_name UNIQUE (name)
			);`,
			expectedTable: "products",
			expectedCols:  []string{"id", "name", "price"},
			expectedError: false,
		},
		{
			name: "Invalid CREATE TABLE syntax",
			sql: `CREATE TABLE invalid syntax (
				bad column
			);`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := p.parseCreateTable(tt.sql)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, node)
			assert.Equal(t, types.NodeTable, node.Type)
			assert.Equal(t, tt.expectedTable, node.Name)

			var colNames []string
			for _, child := range node.Children {
				if child.Type == types.NodeColumn {
					colNames = append(colNames, child.Name)
				}
			}
			assert.ElementsMatch(t, tt.expectedCols, colNames)
		})
	}
}

func TestParseColumnDefinition(t *testing.T) {
	p := NewParser("postgres", ParserOptions{})

	tests := []struct {
		name         string
		columnDef    string
		expectedName string
		expectedType string
		expectedNull bool
		shouldBeNil  bool
	}{
		{
			name:         "Basic integer column",
			columnDef:    "id INTEGER",
			expectedName: "id",
			expectedType: "INTEGER",
			expectedNull: true,
		},
		{
			name:         "VARCHAR with length",
			columnDef:    "name VARCHAR(255) NOT NULL",
			expectedName: "name",
			expectedType: "VARCHAR(255)",
			expectedNull: false,
		},
		{
			name:         "Quoted identifier",
			columnDef:    `"user_id" BIGINT REFERENCES users(id)`,
			expectedName: "user_id",
			expectedType: "BIGINT",
			expectedNull: true,
		},
		{
			name:        "Empty definition",
			columnDef:   "",
			shouldBeNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := p.ParseColumnDefinition(tt.columnDef)

			if tt.shouldBeNil {
				assert.Nil(t, node)
				return
			}

			assert.NotNil(t, node)
			assert.Equal(t, types.NodeColumn, node.Type)
			assert.Equal(t, tt.expectedName, node.Name)
			assert.Equal(t, tt.expectedType, node.Metadata["fullType"])
		})
	}
}

func TestParseAlterTable(t *testing.T) {
	p := NewParser("postgres", ParserOptions{})

	tests := []struct {
		name          string
		sql           string
		expectedTable string
		expectedError bool
	}{
		{
			name:          "Add column",
			sql:           "ALTER TABLE users ADD COLUMN age INTEGER;",
			expectedTable: "users",
			expectedError: false,
		},
		{
			name:          "Alter column type",
			sql:           "ALTER TABLE products ALTER COLUMN price TYPE NUMERIC(12,2);",
			expectedTable: "products",
			expectedError: false,
		},
		{
			name:          "Invalid ALTER syntax",
			sql:           "ALTER TABLE;",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := p.parseAlterTable(tt.sql)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, node)
			assert.Equal(t, types.NodeTable, node.Type)
			assert.Equal(t, tt.expectedTable, node.Name)
		})
	}
}

func TestSplitStatements(t *testing.T) {
	p := NewParser("postgres", ParserOptions{})

	tests := []struct {
		name          string
		sql           string
		expectedCount int
		expectedTypes []string
	}{
		{
			name: "Multiple statements",
			sql: `
				CREATE TABLE users (id SERIAL PRIMARY KEY);
				ALTER TABLE users ADD COLUMN name TEXT;
				CREATE TABLE posts (id SERIAL PRIMARY KEY);
			`,
			expectedCount: 3,
			expectedTypes: []string{"CREATE", "ALTER", "CREATE"},
		},
		{
			name: "Statements with comments",
			sql: `
				-- Create users table
				CREATE TABLE users (id SERIAL PRIMARY KEY);
				/* Add user name
				   as a new column */
				ALTER TABLE users ADD COLUMN name TEXT;
			`,
			expectedCount: 2,
			expectedTypes: []string{"CREATE", "ALTER"},
		},
		{
			name:          "Empty input",
			sql:           "",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statements := p.splitStatements(tt.sql)
			assert.Equal(t, tt.expectedCount, len(statements))

			if tt.expectedTypes != nil {
				for i, stmt := range statements {
					assert.True(t, strings.HasPrefix(strings.TrimSpace(strings.ToUpper(stmt)), tt.expectedTypes[i]))
				}
			}
		})
	}
}

func TestParse(t *testing.T) {
	p := NewParser("postgres", ParserOptions{})

	tests := []struct {
		name          string
		sql           string
		expectedError bool
		tableCount    int
	}{
		{
			name: "Complete schema",
			sql: `
				CREATE TABLE users (
					id SERIAL PRIMARY KEY,
					name VARCHAR(255) NOT NULL,
					email TEXT UNIQUE
				);

				CREATE TABLE posts (
					id SERIAL PRIMARY KEY,
					user_id INTEGER REFERENCES users(id),
					title TEXT NOT NULL,
					content TEXT
				);

				ALTER TABLE users ADD COLUMN created_at TIMESTAMP;
			`,
			expectedError: false,
			tableCount:    2,
		},
		{
			name:          "Invalid SQL",
			sql:           "INVALID SQL STATEMENT;",
			expectedError: false, // Parser is lenient by default
			tableCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := p.Parse(tt.sql)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, tree)
			assert.Equal(t, tt.tableCount, len(tree.Root.Children))
		})
	}
}

func TestParseCreateIndex(t *testing.T) {
	p := NewParser("postgres", ParserOptions{})

	tests := []struct {
		name           string
		sql            string
		expectedName   string
		expectedTable  string
		expectedCols   string
		expectedUnique bool
		expectedError  bool
	}{
		{
			name:           "Simple index",
			sql:            "CREATE INDEX idx_users_email ON users (email);",
			expectedName:   "idx_users_email",
			expectedTable:  "users",
			expectedCols:   "email",
			expectedUnique: false,
			expectedError:  false,
		},
		{
			name:           "Unique index",
			sql:            "CREATE UNIQUE INDEX idx_users_unique_email ON users (email);",
			expectedName:   "idx_users_unique_email",
			expectedTable:  "users",
			expectedCols:   "email",
			expectedUnique: true,
			expectedError:  false,
		},
		{
			name:           "Multi-column index",
			sql:            "CREATE INDEX idx_users_name_email ON users (first_name, last_name, email);",
			expectedName:   "idx_users_name_email",
			expectedTable:  "users",
			expectedCols:   "first_name, last_name, email",
			expectedUnique: false,
			expectedError:  false,
		},
		{
			name:           "Index with IF NOT EXISTS",
			sql:            "CREATE INDEX IF NOT EXISTS idx_users_status ON users (status);",
			expectedName:   "idx_users_status",
			expectedTable:  "users",
			expectedCols:   "status",
			expectedUnique: false,
			expectedError:  false,
		},
		{
			name:          "Invalid index syntax",
			sql:           "CREATE INDEX invalid_syntax;",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := p.parseCreateIndex(tt.sql)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, node)
			assert.Equal(t, types.NodeIndex, node.Type)
			assert.Equal(t, tt.expectedName, node.Name)
			assert.Equal(t, tt.expectedTable, node.Metadata["table"])
			assert.Equal(t, tt.expectedCols, node.Metadata["columns"])
			assert.Equal(t, tt.expectedUnique, node.Metadata["is_unique"])
			assert.Equal(t, strings.TrimRight(tt.sql, ";"), node.Metadata["original_sql"])
		})
	}
}
