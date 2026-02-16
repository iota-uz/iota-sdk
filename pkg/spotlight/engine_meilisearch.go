package spotlight

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/meilisearch/meilisearch-go"
)

type MeilisearchEngine struct {
	client meilisearch.ServiceManager
	index  string
}

var _ IndexEngine = (*MeilisearchEngine)(nil)

func NewMeilisearchEngine(url, apiKey string) *MeilisearchEngine {
	client := meilisearch.New(url, meilisearch.WithAPIKey(apiKey))
	return &MeilisearchEngine{
		client: client,
		index:  "spotlight",
	}
}

func (e *MeilisearchEngine) Upsert(ctx context.Context, docs []SearchDocument) error {
	if len(docs) == 0 {
		return nil
	}

	// Convert SearchDocument to map for Meilisearch
	documents := make([]map[string]interface{}, len(docs))
	for i, doc := range docs {
		documents[i] = map[string]interface{}{
			"id":          fmt.Sprintf("%s:%s", doc.TenantID.String(), doc.ID),
			"tenant_id":   doc.TenantID.String(),
			"provider":    doc.Provider,
			"entity_type": doc.EntityType,
			"title":       doc.Title,
			"body":        doc.Body,
			"url":         doc.URL,
			"language":    doc.Language,
			"metadata":    doc.Metadata,
			"access":      doc.Access,
		}
	}

	idx := e.client.Index(e.index)
	_, err := idx.AddDocuments(documents, nil)
	if err != nil {
		return fmt.Errorf("meilisearch upsert: %w", err)
	}

	return nil
}

func (e *MeilisearchEngine) Delete(ctx context.Context, refs []DocumentRef) error {
	if len(refs) == 0 {
		return nil
	}

	// Convert refs to document IDs
	ids := make([]string, len(refs))
	for i, ref := range refs {
		ids[i] = fmt.Sprintf("%s:%s", ref.TenantID.String(), ref.ID)
	}

	idx := e.client.Index(e.index)
	_, err := idx.DeleteDocuments(ids, nil)
	if err != nil {
		return fmt.Errorf("meilisearch delete: %w", err)
	}

	return nil
}

func (e *MeilisearchEngine) Search(ctx context.Context, req SearchRequest) ([]SearchHit, error) {
	idx := e.client.Index(e.index)

	// Build filter for tenant isolation
	filter := fmt.Sprintf("tenant_id = %s", req.TenantID.String())

	searchReq := &meilisearch.SearchRequest{
		Filter: filter,
		Limit:  int64(req.TopK),
	}

	// Add additional filters if provided
	if len(req.Filters) > 0 {
		for key, value := range req.Filters {
			filter = fmt.Sprintf("%s AND %s = %s", filter, key, value)
		}
		searchReq.Filter = filter
	}

	resp, err := idx.Search(req.Query, searchReq)
	if err != nil {
		return nil, fmt.Errorf("meilisearch search: %w", err)
	}

	// Convert results to SearchHit
	hits := make([]SearchHit, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		doc := e.convertToSearchDocument(hit)
		hits = append(hits, SearchHit{
			Document:     doc,
			LexicalScore: 1.0, // Meilisearch doesn't expose raw scores in the same way
			FinalScore:   1.0,
		})
	}

	return hits, nil
}

func (e *MeilisearchEngine) Health(ctx context.Context) error {
	_, err := e.client.Health()
	if err != nil {
		return fmt.Errorf("meilisearch health check: %w", err)
	}
	return nil
}

func (e *MeilisearchEngine) convertToSearchDocument(hit interface{}) SearchDocument {
	doc := SearchDocument{}

	hitMap, ok := hit.(map[string]interface{})
	if !ok {
		return doc
	}

	if id, ok := hitMap["id"].(string); ok {
		doc.ID = id
	}
	if tenantID, ok := hitMap["tenant_id"].(string); ok {
		doc.TenantID, _ = uuid.Parse(tenantID)
	}
	if provider, ok := hitMap["provider"].(string); ok {
		doc.Provider = provider
	}
	if entityType, ok := hitMap["entity_type"].(string); ok {
		doc.EntityType = entityType
	}
	if title, ok := hitMap["title"].(string); ok {
		doc.Title = title
	}
	if body, ok := hitMap["body"].(string); ok {
		doc.Body = body
	}
	if url, ok := hitMap["url"].(string); ok {
		doc.URL = url
	}
	if language, ok := hitMap["language"].(string); ok {
		doc.Language = language
	}
	if metadata, ok := hitMap["metadata"].(map[string]string); ok {
		doc.Metadata = metadata
	}

	return doc
}
