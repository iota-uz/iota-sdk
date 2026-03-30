// Package spotlight provides the Meilisearch-backed index engine used by
// Spotlight search and indexing flows.
package spotlight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/meilisearch/meilisearch-go"
)

var _ IndexEngine = (*MeilisearchEngine)(nil)

const IndexSchemaVersion = "2026-03-30-search-v4"

type MeilisearchEngine struct {
	client      meilisearch.ServiceManager
	indexName   string
	setupMu     sync.Mutex
	searchReady atomic.Bool
	writeReady  atomic.Bool
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
	if e.writeReady.Load() {
		return nil
	}
	e.setupMu.Lock()
	defer e.setupMu.Unlock()
	if e.writeReady.Load() {
		return nil
	}
	if err := e.ensureIndexConfigured(); err != nil {
		return err
	}
	e.searchReady.Store(true)
	e.writeReady.Store(true)
	return nil
}

func (e *MeilisearchEngine) setupForSearch() error {
	if e.searchReady.Load() {
		return nil
	}
	e.setupMu.Lock()
	defer e.setupMu.Unlock()
	if e.searchReady.Load() {
		return nil
	}
	created, err := e.ensureIndexExists()
	if err != nil {
		return serrors.E("spotlight.MeilisearchEngine.setupForSearch", err)
	}
	if created {
		if err := e.configureIndex(); err != nil {
			return err
		}
		e.writeReady.Store(true)
	} else if err := e.validateSearchSettings(); err != nil {
		return serrors.E("spotlight.MeilisearchEngine.setupForSearch", err)
	}
	e.searchReady.Store(true)
	return nil
}

func (e *MeilisearchEngine) ensureIndexConfigured() error {
	if _, err := e.ensureIndexExists(); err != nil {
		return err
	}
	return e.configureIndex()
}

func (e *MeilisearchEngine) ensureIndexExists() (bool, error) {
	const op serrors.Op = "spotlight.MeilisearchEngine.ensureIndexExists"

	if _, err := e.client.GetIndex(e.indexName); err != nil {
		if !isMeiliNotFound(err) {
			return false, serrors.E(op, err)
		}
		taskInfo, err := e.client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        e.indexName,
			PrimaryKey: "pk",
		})
		if err != nil {
			return false, serrors.E(op, err)
		}

		task, err := e.client.WaitForTask(taskInfo.TaskUID, 100*time.Millisecond)
		if err != nil {
			return false, serrors.E(op, err)
		}
		if task.Status == meilisearch.TaskStatusFailed {
			return false, serrors.E(op, fmt.Errorf("create index task failed: %s", task.Error.Message))
		}
		return true, nil
	}
	return false, nil
}

func (e *MeilisearchEngine) configureIndex() error {
	const op serrors.Op = "spotlight.MeilisearchEngine.configureIndex"

	index := e.client.Index(e.indexName)

	filterableAttrs := []interface{}{
		"tenant_id",
		"provider",
		"entity_type",
		"domain",
		"schema_version",
		"exact_terms",
		"access_visibility",
		"owner_id",
		"allowed_users",
		"allowed_roles",
		"allowed_permissions",
	}
	filterTask, err := index.UpdateFilterableAttributes(&filterableAttrs)
	if err != nil {
		return serrors.E(op, err)
	}
	if _, err := e.client.WaitForTask(filterTask.TaskUID, 100*time.Millisecond); err != nil {
		return serrors.E(op, err)
	}

	searchableAttrs := []string{"title", "description", "search_text"}
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

func (e *MeilisearchEngine) validateSearchSettings() error {
	const op serrors.Op = "spotlight.MeilisearchEngine.validateSearchSettings"

	index := e.client.Index(e.indexName)
	settings, err := index.GetSettings()
	if err != nil {
		return serrors.E(op, err)
	}

	if missing := missingStrings(settings.FilterableAttributes, requiredFilterableAttributes()); len(missing) > 0 {
		return serrors.E(op, fmt.Errorf("spotlight index is missing filterable attributes: %s", strings.Join(missing, ", ")))
	}
	if missing := missingStrings(settings.SearchableAttributes, requiredSearchableAttributes()); len(missing) > 0 {
		return serrors.E(op, fmt.Errorf("spotlight index is missing searchable attributes: %s", strings.Join(missing, ", ")))
	}
	if missing := missingStrings(settings.SortableAttributes, requiredSortableAttributes()); len(missing) > 0 {
		return serrors.E(op, fmt.Errorf("spotlight index is missing sortable attributes: %s", strings.Join(missing, ", ")))
	}
	if err := e.validateSchemaVersion(index); err != nil {
		return serrors.E(op, err)
	}

	return nil
}

func requiredFilterableAttributes() []string {
	return []string{
		"tenant_id",
		"provider",
		"entity_type",
		"domain",
		"schema_version",
		"exact_terms",
		"access_visibility",
		"owner_id",
		"allowed_users",
		"allowed_roles",
		"allowed_permissions",
	}
}

func requiredSearchableAttributes() []string {
	return []string{"title", "description", "search_text"}
}

func requiredSortableAttributes() []string {
	return []string{"updated_at"}
}

func (e *MeilisearchEngine) validateSchemaVersion(index meilisearch.IndexManager) error {
	const op serrors.Op = "spotlight.MeilisearchEngine.validateSchemaVersion"

	stats, err := index.GetStats()
	if err != nil {
		return serrors.E(op, err)
	}
	if stats.NumberOfDocuments == 0 {
		return nil
	}

	resp, err := index.Search("", &meilisearch.SearchRequest{
		Filter: fmt.Sprintf(`schema_version = "%s"`, escapeFilterString(IndexSchemaVersion)),
		Limit:  1,
	})
	if err != nil {
		return serrors.E(op, err)
	}
	if resp.EstimatedTotalHits != stats.NumberOfDocuments {
		return serrors.E(op, fmt.Errorf(
			"spotlight index schema mismatch: expected %d documents at schema %s, found %d",
			stats.NumberOfDocuments,
			IndexSchemaVersion,
			resp.EstimatedTotalHits,
		))
	}

	return nil
}

func isMeiliNotFound(err error) bool {
	var meiliErr *meilisearch.Error
	return errors.As(err, &meiliErr) && meiliErr.StatusCode == http.StatusNotFound
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
			"pk":             meiliPK(doc.TenantID.String(), doc.ID),
			"id":             doc.ID,
			"tenant_id":      doc.TenantID.String(),
			"provider":       doc.Provider,
			"entity_type":    doc.EntityType,
			"domain":         string(normalizeDomain(doc.Domain, doc.EntityType)),
			"title":          doc.Title,
			"description":    doc.Description,
			"body":           doc.Body,
			"search_text":    coalesceSearchText(doc),
			"exact_terms":    normalizeExactTerms(doc.ExactTerms),
			"url":            doc.URL,
			"language":       doc.Language,
			"metadata":       doc.Metadata,
			"schema_version": IndexSchemaVersion,
			"updated_at":     doc.UpdatedAt.Unix(),
		}

		// Include access policy
		record["access_policy"] = doc.Access
		record["access_visibility"] = string(doc.Access.Visibility)
		record["owner_id"] = doc.Access.OwnerID
		record["allowed_users"] = doc.Access.AllowedUsers
		record["allowed_roles"] = doc.Access.AllowedRoles
		record["allowed_permissions"] = doc.Access.AllowedPermissions

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
	task, err := e.client.Index(e.indexName).AddDocuments(records, &meilisearch.DocumentOptions{
		PrimaryKey: &pk,
	})
	if err != nil {
		return serrors.E(op, err)
	}
	if _, err := e.client.WaitForTask(task.TaskUID, 100*time.Millisecond); err != nil {
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
	task, err := index.DeleteDocuments(pks, nil)
	if err != nil {
		return serrors.E(op, err)
	}
	if _, err := e.client.WaitForTask(task.TaskUID, 100*time.Millisecond); err != nil {
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

	if err := e.setupForSearch(); err != nil {
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

	baseFilter := buildSearchFilter(req)

	if req.Mode == QueryModeLookup && len(req.ExactTerms) > 0 {
		exactFilter := buildExactTermsFilter(req.ExactTerms)
		if exactFilter == "" {
			return e.searchOnce(req, req.Query, baseFilter, topK)
		}
		exactHits, err := e.searchOnce(req, "", appendFilter(baseFilter, exactFilter), topK)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		if len(exactHits) >= topK || strings.TrimSpace(req.Query) == "" {
			return exactHits, nil
		}

		fallbackHits, err := e.searchOnce(req, req.Query, baseFilter, topK)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		return mergeHits(exactHits, fallbackHits, topK), nil
	}

	hits, err := e.searchOnce(req, req.Query, baseFilter, topK)
	if err != nil {
		return nil, serrors.E(op, err)
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

func (e *MeilisearchEngine) searchOnce(req SearchRequest, query, filter string, limit int) ([]SearchHit, error) {
	searchReq := &meilisearch.SearchRequest{
		Filter:           filter,
		Limit:            int64(limit),
		ShowRankingScore: true,
	}
	if len(req.QueryEmbedding) > 0 {
		searchReq.Hybrid = &meilisearch.SearchRequestHybrid{
			SemanticRatio: 0.5,
			Embedder:      "default",
		}
		searchReq.Vector = req.QueryEmbedding
	}
	resp, err := e.client.Index(e.indexName).Search(query, searchReq)
	if err != nil {
		return nil, err
	}
	hits := make([]SearchHit, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		searchHit, err := parseMeiliHit(hit)
		if err != nil {
			return nil, err
		}
		if query == "" && len(req.ExactTerms) > 0 {
			searchHit.WhyMatched = "exact_terms"
			searchHit.FinalScore += 10
		}
		hits = append(hits, searchHit)
	}
	return hits, nil
}

func buildSearchFilter(req SearchRequest) string {
	filters := []string{
		fmt.Sprintf(`tenant_id = "%s"`, req.TenantID.String()),
	}
	if len(req.PreferredDomains) > 0 {
		domainFilters := make([]string, 0, len(req.PreferredDomains))
		for _, domain := range req.PreferredDomains {
			domainFilters = append(domainFilters, fmt.Sprintf(`domain = "%s"`, escapeFilterString(string(domain))))
		}
		filters = append(filters, "("+strings.Join(domainFilters, " OR ")+")")
	}
	if genericFilter := buildGenericFilterClauses(req.Filters); genericFilter != "" {
		filters = append(filters, genericFilter)
	}
	if accessFilter := buildAccessFilter(req); accessFilter != "" {
		filters = append(filters, "("+accessFilter+")")
	}
	return strings.Join(filters, " AND ")
}

func buildGenericFilterClauses(filters map[string]string) string {
	if len(filters) == 0 {
		return ""
	}
	keys := make([]string, 0, len(filters))
	for key, value := range filters {
		if strings.TrimSpace(value) == "" {
			continue
		}
		switch key {
		case "provider", "entity_type", "domain", "language":
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	clauses := make([]string, 0, len(keys))
	for _, key := range keys {
		clauses = append(clauses, fmt.Sprintf(`%s = "%s"`, key, escapeFilterString(filters[key])))
	}
	return strings.Join(clauses, " AND ")
}

func buildAccessFilter(req SearchRequest) string {
	clauses := []string{`access_visibility = "public"`}
	if req.UserID != "" {
		escapedUserID := escapeFilterString(req.UserID)
		clauses = append(clauses,
			fmt.Sprintf(`(access_visibility = "owner" AND owner_id = "%s")`, escapedUserID),
			fmt.Sprintf(`allowed_users = "%s"`, escapedUserID),
		)
	}
	for _, role := range dedupeAndSort(req.Roles) {
		clauses = append(clauses, fmt.Sprintf(`allowed_roles = "%s"`, escapeFilterString(role)))
	}
	for _, permission := range dedupeAndSort(req.Permissions) {
		clauses = append(clauses, fmt.Sprintf(`allowed_permissions = "%s"`, escapeFilterString(permission)))
	}
	return strings.Join(clauses, " OR ")
}

func buildExactTermsFilter(terms []string) string {
	normalized := normalizeExactTerms(terms)
	if len(normalized) == 0 {
		return ""
	}
	clauses := make([]string, 0, len(normalized))
	for _, term := range normalized {
		clauses = append(clauses, fmt.Sprintf(`exact_terms = "%s"`, escapeFilterString(term)))
	}
	return "(" + strings.Join(clauses, " OR ") + ")"
}

func appendFilter(base, extra string) string {
	if base == "" {
		return extra
	}
	if extra == "" {
		return base
	}
	return base + " AND " + extra
}

func coalesceSearchText(doc SearchDocument) string {
	if strings.TrimSpace(doc.SearchText) != "" {
		return doc.SearchText
	}
	if strings.TrimSpace(doc.Body) != "" {
		return doc.Body
	}
	return BuildSearchText(doc.Title, doc.Description)
}

func normalizeExactTerms(values []string) []string {
	normalized := ExpandExactTerms(values...)
	slices.Sort(normalized)
	return slices.Compact(normalized)
}

func missingStrings(actual, required []string) []string {
	set := make(map[string]struct{}, len(actual))
	for _, value := range actual {
		set[value] = struct{}{}
	}
	missing := make([]string, 0, len(required))
	for _, value := range required {
		if _, ok := set[value]; !ok {
			missing = append(missing, value)
		}
	}
	return missing
}

func mergeHits(primary, secondary []SearchHit, limit int) []SearchHit {
	if len(primary) >= limit {
		return primary[:limit]
	}
	out := make([]SearchHit, 0, min(limit, len(primary)+len(secondary)))
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	for _, hit := range primary {
		seen[hit.Document.ID] = struct{}{}
		out = append(out, hit)
	}
	for _, hit := range secondary {
		if _, exists := seen[hit.Document.ID]; exists {
			continue
		}
		out = append(out, hit)
		if len(out) == limit {
			break
		}
	}
	return out
}

func escapeFilterString(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
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

	// Extract Domain
	if domain, ok := hitMap["domain"].(string); ok {
		doc.Domain = ResultDomain(domain)
	}

	// Extract Title
	if title, ok := hitMap["title"].(string); ok {
		doc.Title = title
	}

	// Extract Body
	if body, ok := hitMap["body"].(string); ok {
		doc.Body = body
	}

	// Extract Description
	if description, ok := hitMap["description"].(string); ok {
		doc.Description = description
	}

	// Extract SearchText
	if searchText, ok := hitMap["search_text"].(string); ok {
		doc.SearchText = searchText
	}

	// Extract ExactTerms
	if exactTerms, ok := hitMap["exact_terms"].([]interface{}); ok {
		doc.ExactTerms = make([]string, 0, len(exactTerms))
		for _, raw := range exactTerms {
			if term, ok := raw.(string); ok {
				doc.ExactTerms = append(doc.ExactTerms, term)
			}
		}
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
