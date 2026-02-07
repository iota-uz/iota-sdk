package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/bichat/learning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockValidatedQueryStore struct {
	records         map[uuid.UUID]map[string]learning.ValidatedQuery
	deleteByTenantN int
}

func newMockValidatedQueryStore() *mockValidatedQueryStore {
	return &mockValidatedQueryStore{
		records: map[uuid.UUID]map[string]learning.ValidatedQuery{},
	}
}

func (m *mockValidatedQueryStore) Save(ctx context.Context, query learning.ValidatedQuery) error {
	if _, ok := m.records[query.TenantID]; !ok {
		m.records[query.TenantID] = map[string]learning.ValidatedQuery{}
	}
	m.records[query.TenantID][query.SQL] = query
	return nil
}

func (m *mockValidatedQueryStore) Search(ctx context.Context, question string, opts learning.ValidatedQuerySearchOpts) ([]learning.ValidatedQuery, error) {
	out := make([]learning.ValidatedQuery, 0, len(m.records[opts.TenantID]))
	for _, record := range m.records[opts.TenantID] {
		out = append(out, record)
	}
	return out, nil
}

func (m *mockValidatedQueryStore) IncrementUsage(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockValidatedQueryStore) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockValidatedQueryStore) DeleteByTenant(ctx context.Context, tenantID uuid.UUID) error {
	m.deleteByTenantN++
	delete(m.records, tenantID)
	return nil
}

type mockKBIndexer struct {
	indexCalls   int
	rebuildCalls int
	lastDocs     []kb.Document
}

func (m *mockKBIndexer) IndexDocument(ctx context.Context, doc kb.Document) error {
	m.lastDocs = []kb.Document{doc}
	return nil
}

func (m *mockKBIndexer) IndexDocuments(ctx context.Context, docs []kb.Document) error {
	m.indexCalls++
	m.lastDocs = docs
	return nil
}

func (m *mockKBIndexer) DeleteDocument(ctx context.Context, id string) error {
	return nil
}

func (m *mockKBIndexer) Rebuild(ctx context.Context, source kb.DocumentSource) error {
	m.rebuildCalls++
	docs, err := source.List(ctx)
	if err != nil {
		return err
	}
	m.lastDocs = docs
	return nil
}

func (m *mockKBIndexer) GetStats() kb.IndexStats {
	return kb.IndexStats{}
}

func (m *mockKBIndexer) Close() error {
	return nil
}

func TestKnowledgeBootstrapService_Load_Idempotent(t *testing.T) {
	t.Parallel()

	knowledgeDir := createKnowledgeFixture(t)
	metadataDir := filepath.Join(t.TempDir(), "metadata")
	store := newMockValidatedQueryStore()
	indexer := &mockKBIndexer{}
	tenantID := uuid.New()

	service := NewKnowledgeBootstrapService(KnowledgeBootstrapConfig{
		ValidatedQueryStore: store,
		KBIndexer:           indexer,
		MetadataOutputDir:   metadataDir,
		Now: func() time.Time {
			return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		},
	})

	first, err := service.Load(context.Background(), KnowledgeBootstrapRequest{
		TenantID:     tenantID,
		KnowledgeDir: knowledgeDir,
		Rebuild:      false,
	})
	require.NoError(t, err)
	second, err := service.Load(context.Background(), KnowledgeBootstrapRequest{
		TenantID:     tenantID,
		KnowledgeDir: knowledgeDir,
		Rebuild:      false,
	})
	require.NoError(t, err)

	assert.Equal(t, 1, first.TableFilesLoaded)
	assert.Equal(t, 2, first.QueryPatternsLoaded)
	assert.Equal(t, 1, first.BusinessFilesLoaded)
	assert.Equal(t, 4, first.KnowledgeDocsIndexed)
	assert.Equal(t, 1, first.MetadataFilesGenerated)

	assert.Equal(t, 2, second.QueryPatternsLoaded)
	assert.Equal(t, 2, len(store.records[tenantID]), "upsert should keep unique SQL patterns")
	assert.Equal(t, 2, indexer.indexCalls, "load should index each run in upsert mode")
	assert.Equal(t, 0, indexer.rebuildCalls)
}

func TestKnowledgeBootstrapService_Load_Rebuild(t *testing.T) {
	t.Parallel()

	knowledgeDir := createKnowledgeFixture(t)
	store := newMockValidatedQueryStore()
	indexer := &mockKBIndexer{}
	tenantID := uuid.New()

	service := NewKnowledgeBootstrapService(KnowledgeBootstrapConfig{
		ValidatedQueryStore: store,
		KBIndexer:           indexer,
	})

	_, err := service.Load(context.Background(), KnowledgeBootstrapRequest{
		TenantID:     tenantID,
		KnowledgeDir: knowledgeDir,
		Rebuild:      true,
	})
	require.NoError(t, err)

	assert.Equal(t, 1, store.deleteByTenantN)
	assert.Equal(t, 1, indexer.rebuildCalls)
	assert.Len(t, indexer.lastDocs, 4)
}

func createKnowledgeFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "tables"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "queries"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "business"), 0755))

	tableJSON := `{
  "table_name": "sales",
  "table_description": "Sales facts",
  "use_cases": ["Revenue trend"],
  "data_quality_notes": ["Backfill lag by 1 day"],
  "table_columns": [{"name":"amount","description":"Sale amount"}]
}`
	querySQL := `-- <query name>sales_by_month</query name>
-- <query description>
-- Monthly sales totals
-- </query description>
-- <query>
SELECT date_trunc('month', created_at) AS month, sum(amount) AS revenue
FROM sales
GROUP BY 1
ORDER BY 1
-- </query>

-- <query name>top_customers</query name>
-- <query description>
-- Top customers by sales
-- </query description>
-- <query>
SELECT customer_id, sum(amount) AS revenue
FROM sales
GROUP BY 1
ORDER BY revenue DESC
LIMIT 10
-- </query>
`
	businessJSON := `{"business_rules":["Recognize revenue on paid invoices"]}`

	require.NoError(t, os.WriteFile(filepath.Join(root, "tables", "sales.json"), []byte(tableJSON), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "queries", "patterns.sql"), []byte(querySQL), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "business", "rules.json"), []byte(businessJSON), 0644))

	return root
}
