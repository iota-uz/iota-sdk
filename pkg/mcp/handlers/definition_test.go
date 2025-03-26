package handlers

import (
	"context"
	"go/token"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestGetDefinition(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		expectedError bool
		errorContains string
	}{
		{
			name:          "Missing path parameter",
			path:          "",
			expectedError: true,
			errorContains: "path parameter is required",
		},
		{
			name:          "Invalid path format",
			path:          "invalidpath",
			expectedError: true,
			errorContains: "invalid path format",
		},
		{
			name:          "Package not part of project",
			path:          "github.com/unknown/repo.Symbol",
			expectedError: true,
			errorContains: "not part of this project",
		},
		{
			name:          "Symbol not found",
			path:          "github.com/iota-uz/iota-sdk/pkg/mcp/handlers.NonExistentSymbol",
			expectedError: true,
			errorContains: "symbol NonExistentSymbol not found",
		},
		{
			name:          "Valid path",
			path:          "github.com/iota-uz/iota-sdk/pkg/mcp/handlers.GetDefinition",
			expectedError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a request
			args := make(map[string]interface{})
			if tc.path != "" {
				args["path"] = tc.path
			}
			
			request := mcp.CallToolRequest{}
			request.Params.Name = "get_definition"
			request.Params.Arguments = args

			// Call the handler
			result, err := GetDefinition(context.Background(), request)

			// Check the result
			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotNil(t, result.Result)
			}
		})
	}
}

func TestFormatFunctions(t *testing.T) {
	// These are more limited tests since they require AST nodes
	// which are not easy to create manually for testing
	
	t.Run("nodeToString with nil node", func(t *testing.T) {
		// This should not panic
		fset := token.NewFileSet()
		result := nodeToString(fset, nil)
		assert.Equal(t, "", result)
	})
}