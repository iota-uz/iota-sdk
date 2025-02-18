package ast

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/schema/types"
	"github.com/stretchr/testify/assert"
)

func TestNewSchemaTree(t *testing.T) {
	tests := []struct {
		name string
		want *types.SchemaTree
	}{
		{
			name: "creates new schema tree with empty root node",
			want: &types.SchemaTree{
				Root: &types.Node{
					Type:     types.NodeRoot,
					Children: make([]*types.Node, 0),
					Metadata: make(map[string]interface{}),
				},
				Metadata: make(map[string]interface{}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewSchemaTree()

			// Verify the root node type
			assert.Equal(t, tt.want.Root.Type, got.Root.Type)

			// Verify children slice is initialized
			assert.NotNil(t, got.Root.Children)
			assert.Len(t, got.Root.Children, 0)

			// Verify metadata maps are initialized
			assert.NotNil(t, got.Root.Metadata)
			assert.Len(t, got.Root.Metadata, 0)
			assert.NotNil(t, got.Metadata)
			assert.Len(t, got.Metadata, 0)
		})
	}
}
