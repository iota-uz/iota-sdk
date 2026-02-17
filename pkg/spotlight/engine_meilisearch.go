package spotlight

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/meilisearch/meilisearch-go"
)

var _ IndexEngine = (*MeilisearchEngine)(nil)

type MeilisearchEngine struct {
	client    meilisearch.ServiceManager
	indexName string
	setupMu   sync.Mutex
	setupDone atomic.Bool
}

// NewMeilisearchEngine creates a new Meilisearch-based IndexEngine.
func NewMeilisearchEngine(url, apiKey string) *MeilisearchEngine {
	return &MeilisearchEngine{
		client:    meilisearch.New(url, meilisearch.WithAPIKey(apiKey)),
		indexName: "spotlight",
	}
}

// setup ensures the index is created and configured, retrying on failure.
func (e *MeilisearchEngine) setup() error {
	if e.setupDone.Load() {
		return nil
	}
	e.setupMu.Lock()
	defer e.setupMu.Unlock()
	if e.setupDone.Load() {
		return nil
	}
	if err := e.ensureIndex(); err != nil {
		return err
	}
	e.setupDone.Store(true)
	return nil
}

// ensureIndex creates and configures the Meilisearch index.
func (e *MeilisearchEngine) ensureIndex() error {
	const op serrors.Op = "spotlight.MeilisearchEngine.ensureIndex"

	taskInfo, err := e.client.CreateIndex(&meilisearch.IndexConfig{
		Uid:        e.indexName,
		PrimaryKey: "pk",
	})
	if err != nil {
		return serrors.E(op, err)
	}

	task, err := e.client.WaitForTask(taskInfo.TaskUID, 100*time.Millisecond)
	if err != nil {
		return serrors.E(op, err)
	}
	if task.Status == meilisearch.TaskStatusFailed {
		// "index_already_exists" is expected on restart
		if task.Error.Code != "index_already_exists" {
			return serrors.E(op, fmt.Errorf("create index task failed: %s", task.Error.Message))
		}
	}

	index := e.client.Index(e.indexName)

	filterableAttrs := []interface{}{"tenant_id", "provider", "entity_type"}
	filterTask, err := index.UpdateFilterableAttributes(&filterableAttrs)
	if err != nil {
		return serrors.E(op, err)
	}
	if _, err := e.client.WaitForTask(filterTask.TaskUID, 100*time.Millisecond); err != nil {
		return serrors.E(op, err)
	}

	searchableAttrs := []string{"title", "body"}
	searchTask, err := index.UpdateSearchableAttributes(&searchableAttrs)
	if err != nil {
		return serrors.E(op, err)
	}
	if _, err := e.client.WaitForTask(searchTask.TaskUID, 100*time.Millisecond); err != nil {
		return serrors.E(op, err)
	}

	sortableAttrs := []string{"updated_at"}
	sortTask, err := index.UpdateSortableAttributes(&sortableAttrs)
	if err != nil {
		return serrors.E(op, err)
	}
	if _, err := e.client.WaitForTask(sortTask.TaskUID, 100*time.Millisecond); err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// Upsert indexes or updates documents in Meilisearch.
func (e *MeilisearchEngine) Upsert(ctx context.Context, docs []SearchDocument) error {
	const op serrors.Op = "spotlight.MeilisearchEngine.Upsert"

	if len(docs) == 0 {
		return nil
	}

	if err := e.setup(); err != nil {
		return serrors.E(op, err)
	}

	// Convert documents to Meilisearch format
	records := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		record := map[string]interface{}{
			"pk":          meiliPK(doc.TenantID.String(), doc.ID),
			"id":          doc.ID,
			"tenant_id":   doc.TenantID.String(),
			"provider":    doc.Provider,
			"entity_type": doc.EntityType,
			"title":       doc.Title,
			"body":        doc.Body,
			"url":         doc.URL,
			"language":    doc.Language,
			"metadata":    doc.Metadata,
			"updated_at":  doc.UpdatedAt.Unix(),
		}

		// Include access policy
		record["access_policy"] = doc.Access

		// Include embeddings if present
		if len(doc.Embedding) > 0 {
			record["_vectors"] = map[string]interface{}{
				"default": doc.Embedding,
			}
		}

		records = append(records, record)
	}

	// Add documents to index
	pk := "pk"
	_, err := e.client.Index(e.indexName).AddDocuments(records, &meilisearch.DocumentOptions{
		PrimaryKey: &pk,
	})
	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// Delete removes documents from Meilisearch by their references.
func (e *MeilisearchEngine) Delete(ctx context.Context, refs []DocumentRef) error {
	const op serrors.Op = "spotlight.MeilisearchEngine.Delete"

	if len(refs) == 0 {
		return nil
	}

	if err := e.setup(); err != nil {
		return serrors.E(op, err)
	}

	index := e.client.Index(e.indexName)
	pks := make([]string, 0, len(refs))
	for _, ref := range refs {
		pks = append(pks, meiliPK(ref.TenantID.String(), ref.ID))
	}
	_, err := index.DeleteDocuments(pks, nil)
	if err != nil {
		return serrors.E(op, err)
	}
	return nil
}

// Search performs a search query in Meilisearch.
func (e *MeilisearchEngine) Search(ctx context.Context, req SearchRequest) ([]SearchHit, error) {
	const op serrors.Op = "spotlight.MeilisearchEngine.Search"

	if req.TenantID == uuid.Nil {
		return nil, nil
	}

	if err := e.setup(); err != nil {
		return nil, serrors.E(op, err)
	}

	// Normalize TopK
	topK := req.TopK
	if topK <= 0 {
		topK = 20
	}
	if topK > 100 {
		topK = 100
	}

	// Build filter for tenant isolation
	filter := fmt.Sprintf(`tenant_id = "%s"`, req.TenantID.String())

	// Build search request
	searchReq := &meilisearch.SearchRequest{
		Filter:           filter,
		Limit:            int64(topK),
		ShowRankingScore: true,
	}

	// Add hybrid search if embeddings are present
	if len(req.QueryEmbedding) > 0 {
		searchReq.Hybrid = &meilisearch.SearchRequestHybrid{
			SemanticRatio: 0.5,
			Embedder:      "default",
		}
		searchReq.Vector = req.QueryEmbedding
	}

	// Execute search
	resp, err := e.client.Index(e.indexName).Search(req.Query, searchReq)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Convert results
	hits := make([]SearchHit, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		searchHit, err := parseMeiliHit(hit)
		if err != nil {
			return nil, serrors.E(op, err)
		}

		hits = append(hits, searchHit)
	}

	return hits, nil
}

// Health checks if Meilisearch is healthy.
func (e *MeilisearchEngine) Health(ctx context.Context) error {
	const op serrors.Op = "spotlight.MeilisearchEngine.Health"

	_, err := e.client.Health()
	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// meiliPK builds a Meilisearch-safe primary key from tenant ID and document ID.
// Meilisearch only allows alphanumeric characters (a-z A-Z 0-9), hyphens (-),
// and underscores (_) in document identifiers.
func meiliPK(tenantID, docID string) string {
	raw := tenantID + "_" + docID
	var b strings.Builder
	b.Grow(len(raw))
	for _, c := range raw {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			b.WriteRune(c)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// parseMeiliHit extracts a SearchHit from a Meilisearch hit.
func parseMeiliHit(hit meilisearch.Hit) (SearchHit, error) {
	const op serrors.Op = "spotlight.parseMeiliHit"

	// Decode the hit into a map for easier access
	var hitMap map[string]interface{}
	if err := hit.DecodeInto(&hitMap); err != nil {
		return SearchHit{}, serrors.E(op, fmt.Errorf("failed to decode hit: %w", err))
	}

	doc := SearchDocument{}

	// Extract ID
	if id, ok := hitMap["id"].(string); ok {
		doc.ID = id
	}

	// Extract TenantID
	if tenantIDStr, ok := hitMap["tenant_id"].(string); ok {
		tenantID, err := uuid.Parse(tenantIDStr)
		if err != nil {
			return SearchHit{}, serrors.E(op, fmt.Errorf("invalid tenant_id: %w", err))
		}
		doc.TenantID = tenantID
	}

	// Extract Provider
	if provider, ok := hitMap["provider"].(string); ok {
		doc.Provider = provider
	}

	// Extract EntityType
	if entityType, ok := hitMap["entity_type"].(string); ok {
		doc.EntityType = entityType
	}

	// Extract Title
	if title, ok := hitMap["title"].(string); ok {
		doc.Title = title
	}

	// Extract Body
	if body, ok := hitMap["body"].(string); ok {
		doc.Body = body
	}

	// Extract URL
	if url, ok := hitMap["url"].(string); ok {
		doc.URL = url
	}

	// Extract Language
	if language, ok := hitMap["language"].(string); ok {
		doc.Language = language
	}

	// Extract Metadata
	if metadata, ok := hitMap["metadata"].(map[string]interface{}); ok {
		doc.Metadata = make(map[string]string)
		for k, v := range metadata {
			if str, ok := v.(string); ok {
				doc.Metadata[k] = str
			}
		}
	}

	// Extract UpdatedAt from Unix timestamp
	if updatedAtUnix, ok := hitMap["updated_at"].(float64); ok {
		doc.UpdatedAt = time.Unix(int64(updatedAtUnix), 0)
	}

	// Extract AccessPolicy
	if accessPolicyRaw, ok := hitMap["access_policy"]; ok {
		// Marshal and unmarshal to handle nested map conversion
		accessPolicyBytes, err := json.Marshal(accessPolicyRaw)
		if err != nil {
			return SearchHit{}, serrors.E(op, fmt.Errorf("failed to marshal access_policy: %w", err))
		}
		if err := json.Unmarshal(accessPolicyBytes, &doc.Access); err != nil {
			return SearchHit{}, serrors.E(op, fmt.Errorf("failed to unmarshal access_policy: %w", err))
		}
	}

	// Extract ranking score
	var finalScore float64
	if rankingScore, ok := hitMap["_rankingScore"].(float64); ok {
		finalScore = rankingScore
	}

	return SearchHit{
		Document:     doc,
		LexicalScore: 0, // Meilisearch doesn't separate lexical/vector scores
		VectorScore:  0,
		FinalScore:   finalScore,
		WhyMatched:   "meilisearch",
	}, nil
}
