package ast

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/schema/types"
	"github.com/stretchr/testify/assert"
)

func TestNewParser(t *testing.T) {
	tests := []struct {
		name     string
		dialect  string
		options  ParserOptions
		expected *Parser
	}{
		{
			name:    "Create parser with default options",
			dialect: "postgres",
			options: ParserOptions{},
			expected: &Parser{
				dialect: "postgres",
				options: ParserOptions{},
			},
		},
		{
			name:    "Create parser with custom options",
			dialect: "mysql",
			options: ParserOptions{
				StrictMode:     true,
				SkipComments:   true,
				MaxErrors:      5,
				SkipValidation: true,
			},
			expected: &Parser{
				dialect: "mysql",
				options: ParserOptions{
					StrictMode:     true,
					SkipComments:   true,
					MaxErrors:      5,
					SkipValidation: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.dialect, tt.options)
			assert.Equal(t, tt.expected, parser)
		})
	}
}

func TestGetDialect(t *testing.T) {
	tests := []struct {
		name           string
		parser         *Parser
		expectedResult string
	}{
		{
			name: "Get postgres dialect",
			parser: &Parser{
				dialect: "postgres",
				options: ParserOptions{},
			},
			expectedResult: "postgres",
		},
		{
			name: "Get mysql dialect",
			parser: &Parser{
				dialect: "mysql",
				options: ParserOptions{},
			},
			expectedResult: "mysql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.parser.GetDialect()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestParseSQL(t *testing.T) {
	tests := []struct {
		name        string
		sqlContent  string
		expectError bool
		expectNil   bool
	}{
		{
			name:        "Parse empty SQL",
			sqlContent:  "",
			expectError: false,
			expectNil:   false,
		},
		{
			name: "Parse simple CREATE TABLE statement",
			sqlContent: `CREATE TABLE users (
				id SERIAL PRIMARY KEY,
				name VARCHAR(255) NOT NULL
			);`,
			expectError: false,
			expectNil:   false,
		},
		{
			name:        "Parse invalid SQL",
			sqlContent:  "CREATE INVALID SQL",
			expectError: false, // Parser is lenient by default
			expectNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseSQL(tt.sqlContent)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.IsType(t, &types.SchemaTree{}, result)
			}
		})
	}
}
