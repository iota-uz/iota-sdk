package diff

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/schema/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzer_Compare_TableChanges(t *testing.T) {
	tests := []struct {
		name          string
		oldSchema     *types.SchemaTree
		newSchema     *types.SchemaTree
		expectedTypes []ChangeType
	}{
		{
			name: "Add new table",
			oldSchema: &types.SchemaTree{
				Root: &types.Node{
					Type:     types.NodeRoot,
					Children: []*types.Node{},
				},
			},
			newSchema: &types.SchemaTree{
				Root: &types.Node{
					Type: types.NodeRoot,
					Children: []*types.Node{
						{
							Type: types.NodeTable,
							Name: "users",
							Children: []*types.Node{
								{
									Type: types.NodeColumn,
									Name: "id",
									Metadata: map[string]interface{}{
										"type":        "integer",
										"fullType":    "integer",
										"constraints": "primary key",
										"definition":  "id integer primary key",
									},
								},
							},
						},
					},
				},
			},
			expectedTypes: []ChangeType{CreateTable},
		},
		{
			name: "Drop existing table",
			oldSchema: &types.SchemaTree{
				Root: &types.Node{
					Type: types.NodeRoot,
					Children: []*types.Node{
						{
							Type: types.NodeTable,
							Name: "users",
							Children: []*types.Node{
								{
									Type: types.NodeColumn,
									Name: "id",
									Metadata: map[string]interface{}{
										"type":        "integer",
										"fullType":    "integer",
										"constraints": "primary key",
										"definition":  "id integer primary key",
									},
								},
							},
						},
					},
				},
			},
			newSchema: &types.SchemaTree{
				Root: &types.Node{
					Type:     types.NodeRoot,
					Children: []*types.Node{},
				},
			},
			expectedTypes: []ChangeType{DropTable},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewAnalyzer(tt.oldSchema, tt.newSchema, AnalyzerOptions{})
			changes, err := analyzer.Compare()

			assert.NoError(t, err)
			assert.NotNil(t, changes)
			assert.Equal(t, len(tt.expectedTypes), len(changes.Changes))

			for i, expectedType := range tt.expectedTypes {
				assert.Equal(t, expectedType, changes.Changes[i].Type)
			}
		})
	}
}

func TestAnalyzer_Compare_ColumnChanges(t *testing.T) {
	tests := []struct {
		name          string
		oldSchema     *types.SchemaTree
		newSchema     *types.SchemaTree
		expectedTypes []ChangeType
	}{
		{
			name: "Add new column",
			oldSchema: &types.SchemaTree{
				Root: &types.Node{
					Type: types.NodeRoot,
					Children: []*types.Node{
						{
							Type: types.NodeTable,
							Name: "users",
							Children: []*types.Node{
								{
									Type: types.NodeColumn,
									Name: "id",
									Metadata: map[string]interface{}{
										"type":        "integer",
										"fullType":    "integer",
										"constraints": "primary key",
										"definition":  "id integer primary key",
									},
								},
							},
						},
					},
				},
			},
			newSchema: &types.SchemaTree{
				Root: &types.Node{
					Type: types.NodeRoot,
					Children: []*types.Node{
						{
							Type: types.NodeTable,
							Name: "users",
							Children: []*types.Node{
								{
									Type: types.NodeColumn,
									Name: "id",
									Metadata: map[string]interface{}{
										"type":        "integer",
										"fullType":    "integer",
										"constraints": "primary key",
										"definition":  "id integer primary key",
									},
								},
								{
									Type: types.NodeColumn,
									Name: "email",
									Metadata: map[string]interface{}{
										"type":        "varchar",
										"fullType":    "varchar(255)",
										"constraints": "not null unique",
										"definition":  "email varchar(255) not null unique",
									},
								},
							},
						},
					},
				},
			},
			expectedTypes: []ChangeType{AddColumn},
		},
		{
			name: "Modify column type",
			oldSchema: &types.SchemaTree{
				Root: &types.Node{
					Type: types.NodeRoot,
					Children: []*types.Node{
						{
							Type: types.NodeTable,
							Name: "users",
							Children: []*types.Node{
								{
									Type: types.NodeColumn,
									Name: "status",
									Metadata: map[string]interface{}{
										"type":        "varchar",
										"fullType":    "varchar(50)",
										"constraints": "not null",
										"definition":  "status varchar(50) not null",
									},
								},
							},
						},
					},
				},
			},
			newSchema: &types.SchemaTree{
				Root: &types.Node{
					Type: types.NodeRoot,
					Children: []*types.Node{
						{
							Type: types.NodeTable,
							Name: "users",
							Children: []*types.Node{
								{
									Type: types.NodeColumn,
									Name: "status",
									Metadata: map[string]interface{}{
										"type":        "varchar",
										"fullType":    "varchar(100)",
										"constraints": "not null",
										"definition":  "status varchar(100) not null",
									},
								},
							},
						},
					},
				},
			},
			expectedTypes: []ChangeType{ModifyColumn},
		},
		{
			name: "Drop column",
			oldSchema: &types.SchemaTree{
				Root: &types.Node{
					Type: types.NodeRoot,
					Children: []*types.Node{
						{
							Type: types.NodeTable,
							Name: "users",
							Children: []*types.Node{
								{
									Type: types.NodeColumn,
									Name: "id",
									Metadata: map[string]interface{}{
										"type":        "integer",
										"fullType":    "integer",
										"constraints": "primary key",
										"definition":  "id integer primary key",
									},
								},
								{
									Type: types.NodeColumn,
									Name: "temp_field",
									Metadata: map[string]interface{}{
										"type":        "text",
										"fullType":    "text",
										"constraints": "",
										"definition":  "temp_field text",
									},
								},
							},
						},
					},
				},
			},
			newSchema: &types.SchemaTree{
				Root: &types.Node{
					Type: types.NodeRoot,
					Children: []*types.Node{
						{
							Type: types.NodeTable,
							Name: "users",
							Children: []*types.Node{
								{
									Type: types.NodeColumn,
									Name: "id",
									Metadata: map[string]interface{}{
										"type":        "integer",
										"fullType":    "integer",
										"constraints": "primary key",
										"definition":  "id integer primary key",
									},
								},
							},
						},
					},
				},
			},
			expectedTypes: []ChangeType{DropColumn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewAnalyzer(tt.oldSchema, tt.newSchema, AnalyzerOptions{})
			changes, err := analyzer.Compare()

			assert.NoError(t, err)
			assert.NotNil(t, changes)
			assert.Equal(t, len(tt.expectedTypes), len(changes.Changes))

			for i, expectedType := range tt.expectedTypes {
				assert.Equal(t, expectedType, changes.Changes[i].Type)
			}
		})
	}
}

func TestAnalyzer_ColumnsEqual(t *testing.T) {
	tests := []struct {
		name      string
		oldColumn *types.Node
		newColumn *types.Node
		expected  bool
	}{
		{
			name: "Identical columns",
			oldColumn: &types.Node{
				Type: types.NodeColumn,
				Name: "email",
				Metadata: map[string]interface{}{
					"type":        "varchar",
					"fullType":    "varchar(255)",
					"constraints": "not null unique",
					"definition":  "email varchar(255) not null unique",
				},
			},
			newColumn: &types.Node{
				Type: types.NodeColumn,
				Name: "email",
				Metadata: map[string]interface{}{
					"type":        "varchar",
					"fullType":    "varchar(255)",
					"constraints": "not null unique",
					"definition":  "email varchar(255) not null unique",
				},
			},
			expected: true,
		},
		{
			name: "Different varchar lengths",
			oldColumn: &types.Node{
				Type: types.NodeColumn,
				Name: "name",
				Metadata: map[string]interface{}{
					"type":        "varchar",
					"fullType":    "varchar(50)",
					"constraints": "not null",
					"definition":  "name varchar(50) not null",
				},
			},
			newColumn: &types.Node{
				Type: types.NodeColumn,
				Name: "name",
				Metadata: map[string]interface{}{
					"type":        "varchar",
					"fullType":    "varchar(100)",
					"constraints": "not null",
					"definition":  "name varchar(100) not null",
				},
			},
			expected: false,
		},
		{
			name: "Different constraints order",
			oldColumn: &types.Node{
				Type: types.NodeColumn,
				Name: "email",
				Metadata: map[string]interface{}{
					"type":        "varchar",
					"fullType":    "varchar(255)",
					"constraints": "unique not null",
					"definition":  "email varchar(255) unique not null",
				},
			},
			newColumn: &types.Node{
				Type: types.NodeColumn,
				Name: "email",
				Metadata: map[string]interface{}{
					"type":        "varchar",
					"fullType":    "varchar(255)",
					"constraints": "not null unique",
					"definition":  "email varchar(255) not null unique",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewAnalyzer(nil, nil, AnalyzerOptions{})
			result := analyzer.columnsEqual(tt.oldColumn, tt.newColumn)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSetLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		level logrus.Level
	}{
		{"Debug level", logrus.DebugLevel},
		{"Info level", logrus.InfoLevel},
		{"Warn level", logrus.WarnLevel},
		{"Error level", logrus.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLogLevel(tt.level)
			assert.Equal(t, tt.level, logger.GetLevel())
		})
	}
}
