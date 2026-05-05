package sql

import (
	"context"
	"testing"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSchemaDescriber implements bichatsql.SchemaDescriber for testing.
type mockSchemaDescriber struct {
	schema *bichatsql.TableSchema
	err    error
}

func (m *mockSchemaDescriber) SchemaDescribe(ctx context.Context, tableName string) (*bichatsql.TableSchema, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.schema != nil {
		return m.schema, nil
	}
	return &bichatsql.TableSchema{
		Name:   tableName,
		Schema: "public",
		Columns: []bichatsql.ColumnInfo{
			{Name: "id", Type: "integer", IsPrimaryKey: true},
			{Name: "name", Type: "text"},
		},
	}, nil
}

// mockSchemaLister implements bichatsql.SchemaLister for testing.
type mockSchemaLister struct {
	tables []bichatsql.TableInfo
	err    error
}

func (m *mockSchemaLister) SchemaList(ctx context.Context) ([]bichatsql.TableInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.tables != nil {
		return m.tables, nil
	}
	return []bichatsql.TableInfo{
		{Name: "users", Schema: "public", RowCount: 100, Description: "User accounts"},
		{Name: "orders", Schema: "public", RowCount: 1000, Description: "Customer orders"},
	}, nil
}

func TestSchemaListTool(t *testing.T) {
	t.Parallel()

	lister := &mockSchemaLister{
		tables: []bichatsql.TableInfo{
			{Name: "policies", Schema: "analytics", RowCount: 100, Description: "Policy view"},
			{Name: "payments", Schema: "analytics", RowCount: 200, Description: "Payment view"},
		},
	}

	tool := NewSchemaListTool(lister)

	result, err := tool.Call(context.Background(), `{}`)
	require.NoError(t, err)

	assert.Contains(t, result, "policies")
	assert.Contains(t, result, "payments")
	assert.Contains(t, result, "~100")
	assert.Contains(t, result, "~200")
}
