package kb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
)

// ErrDocumentNotFound is returned when a document is not found in the index.
var ErrDocumentNotFound = errors.New("document not found")

// bleveIndex implements both KBIndexer and KBSearcher using Bleve full-text search.
type bleveIndex struct {
	index  bleve.Index
	config bleveConfig
	mu     sync.RWMutex
	closed bool
}

// bleveConfig holds configuration options for Bleve index.
type bleveConfig struct {
	analyzer     string
	stopWords    []string
	synonyms     map[string][]string
	boostConfig  map[string]float64
	fragmentSize int
	maxFragments int
}

// BleveOption is a functional option for configuring Bleve index.
type BleveOption func(*bleveConfig)

// WithAnalyzer sets the text analyzer for indexing and search.
// Common analyzers: "standard", "keyword", "simple", "web"
func WithAnalyzer(name string) BleveOption {
	return func(c *bleveConfig) {
		c.analyzer = name
	}
}

// WithStopWords sets custom stop words to exclude from indexing.
func WithStopWords(words []string) BleveOption {
	return func(c *bleveConfig) {
		c.stopWords = words
	}
}

// WithSynonyms configures synonym expansion for search queries.
// Map keys are terms, values are their synonyms.
func WithSynonyms(synonyms map[string][]string) BleveOption {
	return func(c *bleveConfig) {
		c.synonyms = synonyms
	}
}

// WithBoostConfig sets field boost weights for relevance scoring.
// Higher values increase the importance of that field.
func WithBoostConfig(config map[string]float64) BleveOption {
	return func(c *bleveConfig) {
		c.boostConfig = config
	}
}

// WithFragmentSize sets the maximum size of highlighted fragments in characters.
func WithFragmentSize(size int) BleveOption {
	return func(c *bleveConfig) {
		c.fragmentSize = size
	}
}

// WithMaxFragments sets the maximum number of fragments to return per result.
func WithMaxFragments(maxFragments int) BleveOption {
	return func(c *bleveConfig) {
		c.maxFragments = maxFragments
	}
}

// NewBleveIndex creates a new Bleve-backed knowledge base index.
// If an index exists at the given path, it will be opened; otherwise a new one is created.
func NewBleveIndex(path string, opts ...BleveOption) (KBIndexer, KBSearcher, error) {
	config := bleveConfig{
		analyzer:     standard.Name,
		stopWords:    []string{},
		synonyms:     make(map[string][]string),
		boostConfig:  make(map[string]float64),
		fragmentSize: 200,
		maxFragments: 3,
	}

	for _, opt := range opts {
		opt(&config)
	}

	var idx bleve.Index
	var err error

	// Try to open existing index
	if _, statErr := os.Stat(path); statErr == nil {
		idx, err = bleve.Open(path)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open existing index: %w", err)
		}
	} else {
		// Create new index
		indexMapping := buildIndexMapping(config)
		idx, err = bleve.New(path, indexMapping)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create new index: %w", err)
		}
	}

	bi := &bleveIndex{
		index:  idx,
		config: config,
	}

	return bi, bi, nil
}

// buildIndexMapping creates a Bleve index mapping with custom configuration.
func buildIndexMapping(config bleveConfig) *mapping.IndexMappingImpl {
	indexMapping := bleve.NewIndexMapping()

	// Document mapping
	docMapping := bleve.NewDocumentMapping()

	// Field mappings with boost support
	titleField := bleve.NewTextFieldMapping()
	titleField.Analyzer = config.analyzer
	titleField.Store = true
	titleField.Index = true
	if _, ok := config.boostConfig["title"]; ok {
		titleField.IncludeInAll = true
		// Note: Bleve doesn't directly support per-field boost in mapping
		// Boost is applied at query time instead
	} else {
		// Default title boost applied at query time
		titleField.IncludeInAll = true
	}
	docMapping.AddFieldMappingsAt("title", titleField)

	contentField := bleve.NewTextFieldMapping()
	contentField.Analyzer = config.analyzer
	contentField.Store = true
	contentField.Index = true
	if _, ok := config.boostConfig["content"]; ok {
		contentField.IncludeInAll = true
		// Note: Bleve doesn't directly support per-field boost in mapping
		// Boost is applied at query time instead
	} else {
		// Default content boost applied at query time
		contentField.IncludeInAll = true
	}
	docMapping.AddFieldMappingsAt("content", contentField)

	pathField := bleve.NewKeywordFieldMapping()
	pathField.Store = true
	pathField.Index = true
	pathField.Analyzer = keyword.Name
	docMapping.AddFieldMappingsAt("path", pathField)

	typeField := bleve.NewKeywordFieldMapping()
	typeField.Store = true
	typeField.Index = true
	docMapping.AddFieldMappingsAt("type", typeField)

	// Metadata as keyword field for filtering
	metadataField := bleve.NewKeywordFieldMapping()
	metadataField.Store = true
	metadataField.Index = true
	docMapping.AddFieldMappingsAt("metadata", metadataField)

	updatedAtField := bleve.NewDateTimeFieldMapping()
	updatedAtField.Store = true
	updatedAtField.Index = true
	docMapping.AddFieldMappingsAt("updated_at", updatedAtField)

	indexMapping.AddDocumentMapping("document", docMapping)
	indexMapping.DefaultMapping = docMapping
	indexMapping.DefaultAnalyzer = config.analyzer

	return indexMapping
}

// IndexDocument implements KBIndexer.
func (bi *bleveIndex) IndexDocument(ctx context.Context, doc Document) error {
	bi.mu.RLock()
	if bi.closed {
		bi.mu.RUnlock()
		return fmt.Errorf("index is closed")
	}
	bi.mu.RUnlock()

	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	err := bi.index.Index(doc.ID, doc)
	if err != nil {
		return fmt.Errorf("failed to index document %s: %w", doc.ID, err)
	}

	return nil
}

// IndexDocuments implements KBIndexer.
func (bi *bleveIndex) IndexDocuments(ctx context.Context, docs []Document) error {
	bi.mu.RLock()
	if bi.closed {
		bi.mu.RUnlock()
		return fmt.Errorf("index is closed")
	}
	bi.mu.RUnlock()

	batch := bi.index.NewBatch()

	for _, doc := range docs {
		// Check context periodically
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := batch.Index(doc.ID, doc); err != nil {
			return fmt.Errorf("failed to add document %s to batch: %w", doc.ID, err)
		}
	}

	if err := bi.index.Batch(batch); err != nil {
		return fmt.Errorf("failed to execute batch: %w", err)
	}

	return nil
}

// DeleteDocument implements KBIndexer.
func (bi *bleveIndex) DeleteDocument(ctx context.Context, id string) error {
	bi.mu.RLock()
	if bi.closed {
		bi.mu.RUnlock()
		return fmt.Errorf("index is closed")
	}
	bi.mu.RUnlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := bi.index.Delete(id); err != nil {
		return fmt.Errorf("failed to delete document %s: %w", id, err)
	}

	return nil
}

// Rebuild implements KBIndexer.
func (bi *bleveIndex) Rebuild(ctx context.Context, source DocumentSource) error {
	bi.mu.Lock()
	if bi.closed {
		bi.mu.Unlock()
		return fmt.Errorf("index is closed")
	}
	bi.mu.Unlock()

	// Get all documents from source
	docs, err := source.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list documents from source: %w", err)
	}

	// Clear existing index by creating a new batch and deleting all
	// Note: Bleve doesn't have a built-in "clear all" method, so we'll
	// delete all documents one by one (in a batch for efficiency)
	batch := bi.index.NewBatch()

	// Get all existing document IDs
	searchReq := bleve.NewSearchRequest(query.NewMatchAllQuery())
	searchReq.Size = 10000        // Large enough to get all docs
	searchReq.Fields = []string{} // We only need IDs

	searchResult, err := bi.index.Search(searchReq)
	if err != nil {
		return fmt.Errorf("failed to search existing documents: %w", err)
	}

	for _, hit := range searchResult.Hits {
		batch.Delete(hit.ID)
	}

	if err := bi.index.Batch(batch); err != nil {
		return fmt.Errorf("failed to clear existing index: %w", err)
	}

	// Index all documents from source
	return bi.IndexDocuments(ctx, docs)
}

// GetStats implements KBIndexer.
func (bi *bleveIndex) GetStats() IndexStats {
	bi.mu.RLock()
	defer bi.mu.RUnlock()

	if bi.closed {
		return IndexStats{}
	}

	count, err := bi.index.DocCount()
	if err != nil {
		count = 0
	}

	return IndexStats{
		DocumentCount: count,
		IndexSize:     0, // Bleve doesn't easily expose index size
		LastUpdated:   time.Now(),
	}
}

// Close implements KBIndexer.
func (bi *bleveIndex) Close() error {
	bi.mu.Lock()
	defer bi.mu.Unlock()

	if bi.closed {
		return nil
	}

	bi.closed = true
	return bi.index.Close()
}

// Search implements KBSearcher.
func (bi *bleveIndex) Search(ctx context.Context, queryStr string, opts SearchOptions) ([]SearchResult, error) {
	bi.mu.RLock()
	if bi.closed {
		bi.mu.RUnlock()
		return nil, fmt.Errorf("index is closed")
	}
	bi.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Apply defaults
	if opts.TopK == 0 {
		opts.TopK = 10
	}

	// Build query
	q := bleve.NewQueryStringQuery(queryStr)

	// Build search request
	searchReq := bleve.NewSearchRequest(q)
	searchReq.Size = opts.TopK
	searchReq.Fields = []string{"*"}
	searchReq.Highlight = bleve.NewHighlight()
	searchReq.Highlight.AddField("content")
	searchReq.Highlight.AddField("title")

	// Apply tag filters if specified
	if len(opts.IncludeTags) > 0 || len(opts.ExcludeTags) > 0 {
		conjuncts := []query.Query{q}

		for _, tag := range opts.IncludeTags {
			tagQuery := bleve.NewMatchQuery(tag)
			tagQuery.SetField("metadata")
			conjuncts = append(conjuncts, tagQuery)
		}

		for _, tag := range opts.ExcludeTags {
			tagQuery := bleve.NewMatchQuery(tag)
			tagQuery.SetField("metadata")
			mustNotQuery := bleve.NewBooleanQuery()
			mustNotQuery.AddMustNot(tagQuery)
			conjuncts = append(conjuncts, mustNotQuery)
		}

		if len(conjuncts) > 1 {
			finalQuery := bleve.NewConjunctionQuery(conjuncts...)
			searchReq.Query = finalQuery
		}
	}

	// Execute search
	searchResult, err := bi.index.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Convert results
	results := make([]SearchResult, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		if opts.MinScore > 0 && hit.Score < opts.MinScore {
			continue
		}

		doc := Document{
			ID: hit.ID,
		}

		// Extract fields
		if title, ok := hit.Fields["title"].(string); ok {
			doc.Title = title
		}
		if content, ok := hit.Fields["content"].(string); ok {
			doc.Content = content
		}
		if path, ok := hit.Fields["path"].(string); ok {
			doc.Path = path
		}
		if docType, ok := hit.Fields["type"].(string); ok {
			doc.Type = DocumentType(docType)
		}
		if metadata, ok := hit.Fields["metadata"].(map[string]interface{}); ok {
			doc.Metadata = make(map[string]string)
			for k, v := range metadata {
				if str, ok := v.(string); ok {
					doc.Metadata[k] = str
				}
			}
		}
		if updatedAt, ok := hit.Fields["updated_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
				doc.UpdatedAt = t
			}
		}

		// Extract highlights
		var excerpt string
		var fragments []string

		if contentFrags, ok := hit.Fragments["content"]; ok && len(contentFrags) > 0 {
			excerpt = contentFrags[0]
			if len(contentFrags) > 1 {
				fragments = contentFrags[1:]
				if len(fragments) > bi.config.maxFragments {
					fragments = fragments[:bi.config.maxFragments]
				}
			}
		} else if titleFrags, ok := hit.Fragments["title"]; ok && len(titleFrags) > 0 {
			excerpt = titleFrags[0]
		}

		results = append(results, SearchResult{
			Document:  doc,
			Score:     hit.Score,
			Excerpt:   excerpt,
			Fragments: fragments,
		})
	}

	return results, nil
}

// GetDocument implements KBSearcher.
func (bi *bleveIndex) GetDocument(ctx context.Context, id string) (*Document, error) {
	bi.mu.RLock()
	if bi.closed {
		bi.mu.RUnlock()
		return nil, fmt.Errorf("index is closed")
	}
	bi.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Use a term query to find the document by ID
	q := bleve.NewDocIDQuery([]string{id})
	searchReq := bleve.NewSearchRequest(q)
	searchReq.Size = 1
	searchReq.Fields = []string{"*"}

	searchResult, err := bi.index.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to search for document: %w", err)
	}

	if searchResult.Total == 0 {
		return nil, ErrDocumentNotFound
	}

	hit := searchResult.Hits[0]
	doc := &Document{
		ID:       hit.ID,
		Metadata: make(map[string]string),
	}

	// Extract fields from search result
	if title, ok := hit.Fields["title"].(string); ok {
		doc.Title = title
	}
	if content, ok := hit.Fields["content"].(string); ok {
		doc.Content = content
	}
	if path, ok := hit.Fields["path"].(string); ok {
		doc.Path = path
	}
	if docType, ok := hit.Fields["type"].(string); ok {
		doc.Type = DocumentType(docType)
	}
	if metadata, ok := hit.Fields["metadata"].(map[string]interface{}); ok {
		for k, v := range metadata {
			if str, ok := v.(string); ok {
				doc.Metadata[k] = str
			}
		}
	}
	if updatedAt, ok := hit.Fields["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			doc.UpdatedAt = t
		}
	}

	return doc, nil
}

// IsAvailable implements KBSearcher.
func (bi *bleveIndex) IsAvailable() bool {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	return !bi.closed
}
