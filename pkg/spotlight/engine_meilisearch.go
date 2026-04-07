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

const (
	meiliMaxDocumentsPerRequest = 5000
	// Keep a safety margin below Meilisearch's configured payload ceiling.
	meiliSafePayloadLimitBytes = 90 << 20 // 90 MiB
)

type MeilisearchEngine struct {
	client      meilisearch.ServiceManager
	indexName   string
	activeName  string
	setupMu     sync.Mutex
	searchReady atomic.Bool
	writeReady  atomic.Bool
	pendingMu   sync.Mutex
	pendingUIDs []int64
}

type meiliSearchIndexState struct {
	exists         bool
	documents      int64
	fieldsReady    bool
	schemaVersion  string
	searchableName string
}

type meiliDocumentRecord struct {
	payload   map[string]interface{}
	sizeBytes int
	docID     string
}

// NewMeilisearchEngine creates a new Meilisearch-based IndexEngine.
func NewMeilisearchEngine(url, apiKey string) *MeilisearchEngine {
	return &MeilisearchEngine{
		client:     meilisearch.New(url, meilisearch.WithAPIKey(apiKey)),
		indexName:  "spotlight",
		activeName: "spotlight",
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
	indexName, created, err := e.ensureSearchIndex()
	if err != nil {
		return serrors.E("spotlight.MeilisearchEngine.setupForSearch", err)
	}
	if created {
		if err := e.configureIndex(indexName); err != nil {
			return err
		}
		e.writeReady.Store(true)
	} else if err := e.validateSearchSettings(indexName); err != nil {
		return serrors.E("spotlight.MeilisearchEngine.setupForSearch", err)
	}
	e.searchReady.Store(true)
	return nil
}

func (e *MeilisearchEngine) ensureIndexConfigured() error {
	if _, err := e.ensureIndexExists(e.indexName); err != nil {
		return err
	}
	return e.configureIndex(e.indexName)
}

func (e *MeilisearchEngine) ensureSearchIndex() (string, bool, error) {
	indexName, err := e.resolveSearchIndexName()
	if err != nil {
		return "", false, err
	}
	if indexName != e.indexName {
		return indexName, false, nil
	}
	created, err := e.ensureIndexExists(indexName)
	return indexName, created, err
}

func (e *MeilisearchEngine) ensureIndexExists(indexName string) (bool, error) {
	const op serrors.Op = "spotlight.MeilisearchEngine.ensureIndexExists"

	if _, err := e.client.GetIndex(indexName); err != nil {
		if !isMeiliNotFound(err) {
			return false, serrors.E(op, err)
		}
		taskInfo, err := e.client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        indexName,
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

func (e *MeilisearchEngine) configureIndex(indexName string) error {
	const op serrors.Op = "spotlight.MeilisearchEngine.configureIndex"

	index := e.client.Index(indexName)

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

func (e *MeilisearchEngine) validateSearchSettings(indexName string) error {
	const op serrors.Op = "spotlight.MeilisearchEngine.validateSearchSettings"

	index := e.client.Index(indexName)
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

	return nil
}

func (e *MeilisearchEngine) resolveSearchIndexName() (string, error) {
	if e.indexName != e.activeName {
		return e.indexName, nil
	}

	activeState, err := e.inspectSearchIndex(e.activeName)
	if err != nil {
		return "", err
	}
	if activeState.searchable() {
		return e.activeName, nil
	}

	buildIndexName := rebuildIndexName(e.activeName)
	buildState, err := e.inspectSearchIndex(buildIndexName)
	if err != nil {
		return "", err
	}
	if buildState.currentSchemaReady() {
		return buildIndexName, nil
	}

	return e.activeName, nil
}

func (e *MeilisearchEngine) inspectSearchIndex(indexName string) (meiliSearchIndexState, error) {
	stats, err := e.client.Index(indexName).GetStats()
	if err != nil {
		if isMeiliNotFound(err) {
			return meiliSearchIndexState{}, nil
		}
		return meiliSearchIndexState{}, err
	}
	state := meiliSearchIndexState{
		exists:         true,
		searchableName: indexName,
	}
	if stats == nil {
		return state, nil
	}
	state.documents = stats.NumberOfDocuments
	state.fieldsReady = spotlightIndexFieldsReady(stats.FieldDistribution)
	if state.documents == 0 || !state.fieldsReady {
		return state, nil
	}

	schemaVersion, err := e.indexSchemaVersion(indexName)
	if err != nil {
		return meiliSearchIndexState{}, err
	}
	state.schemaVersion = schemaVersion
	return state, nil
}

func (e *MeilisearchEngine) indexSchemaVersion(indexName string) (string, error) {
	resp, err := e.client.Index(indexName).Search("", &meilisearch.SearchRequest{
		Limit:                1,
		AttributesToRetrieve: []string{"schema_version"},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Hits) == 0 {
		return "", nil
	}
	var hit struct {
		SchemaVersion string `json:"schema_version"`
	}
	payload, err := json.Marshal(resp.Hits[0])
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(payload, &hit); err != nil {
		return "", err
	}
	return hit.SchemaVersion, nil
}

func (s meiliSearchIndexState) searchable() bool {
	return s.exists && s.documents > 0 && s.fieldsReady
}

func (s meiliSearchIndexState) currentSchemaReady() bool {
	return s.searchable() && s.schemaVersion == IndexSchemaVersion
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

func isMeiliNotFound(err error) bool {
	var meiliErr *meilisearch.Error
	return errors.As(err, &meiliErr) && meiliErr.StatusCode == http.StatusNotFound
}

type meiliRebuildSession struct {
	client          meilisearch.ServiceManager
	activeIndexName string
	buildIndexName  string
	engine          *MeilisearchEngine
}

func (s *meiliRebuildSession) Engine() IndexEngine {
	return s.engine
}

func (s *meiliRebuildSession) Commit(ctx context.Context) error {
	if s == nil {
		return nil
	}

	createdPlaceholder := false
	if _, err := s.client.GetIndex(s.activeIndexName); err != nil {
		if !isMeiliNotFound(err) {
			return serrors.E("spotlight.MeilisearchEngine.CommitRebuild", err)
		}
		task, err := s.client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        s.activeIndexName,
			PrimaryKey: "pk",
		})
		if err != nil {
			return serrors.E("spotlight.MeilisearchEngine.CommitRebuild", err)
		}
		if _, err := s.client.WaitForTask(task.TaskUID, 100*time.Millisecond); err != nil {
			return serrors.E("spotlight.MeilisearchEngine.CommitRebuild", err)
		}
		createdPlaceholder = true
	}

	task, err := s.client.SwapIndexesWithContext(ctx, []*meilisearch.SwapIndexesParams{{
		Indexes: []string{s.buildIndexName, s.activeIndexName},
	}})
	if err != nil {
		return serrors.E("spotlight.MeilisearchEngine.CommitRebuild", err)
	}
	if _, err := s.client.WaitForTask(task.TaskUID, 100*time.Millisecond); err != nil {
		return serrors.E("spotlight.MeilisearchEngine.CommitRebuild", err)
	}

	cleanupTask, err := s.client.DeleteIndexWithContext(ctx, s.buildIndexName)
	if err != nil {
		if !createdPlaceholder || !isMeiliNotFound(err) {
			return serrors.E("spotlight.MeilisearchEngine.CommitRebuild", err)
		}
		return nil
	}
	if cleanupTask != nil {
		if _, err := s.client.WaitForTask(cleanupTask.TaskUID, 100*time.Millisecond); err != nil {
			return serrors.E("spotlight.MeilisearchEngine.CommitRebuild", err)
		}
	}

	return nil
}

func (s *meiliRebuildSession) Abort(ctx context.Context) error {
	if s == nil {
		return nil
	}
	task, err := s.client.DeleteIndexWithContext(ctx, s.buildIndexName)
	if err != nil {
		if isMeiliNotFound(err) {
			return nil
		}
		return serrors.E("spotlight.MeilisearchEngine.AbortRebuild", err)
	}
	if task != nil {
		if _, err := s.client.WaitForTask(task.TaskUID, 100*time.Millisecond); err != nil {
			return serrors.E("spotlight.MeilisearchEngine.AbortRebuild", err)
		}
	}
	return nil
}

func (e *MeilisearchEngine) StartRebuild(ctx context.Context) (RebuildSession, error) {
	const op serrors.Op = "spotlight.MeilisearchEngine.StartRebuild"

	buildIndexName := rebuildIndexName(e.activeName)
	if err := e.resetIndex(ctx, buildIndexName); err != nil {
		return nil, serrors.E(op, err)
	}

	buildEngine := &MeilisearchEngine{
		client:     e.client,
		indexName:  buildIndexName,
		activeName: e.activeName,
	}
	if err := buildEngine.setup(); err != nil {
		return nil, serrors.E(op, err)
	}

	return &meiliRebuildSession{
		client:          e.client,
		activeIndexName: e.activeName,
		buildIndexName:  buildIndexName,
		engine:          buildEngine,
	}, nil
}

func (e *MeilisearchEngine) resetIndex(ctx context.Context, indexName string) error {
	if _, err := e.client.GetIndex(indexName); err == nil {
		task, err := e.client.DeleteIndexWithContext(ctx, indexName)
		if err != nil {
			return err
		}
		if task != nil {
			if _, err := e.client.WaitForTask(task.TaskUID, 100*time.Millisecond); err != nil {
				return err
			}
		}
		return nil
	} else if !isMeiliNotFound(err) {
		return err
	}
	return nil
}

func rebuildIndexName(active string) string {
	sanitizedVersion := strings.NewReplacer("-", "_", ".", "_").Replace(IndexSchemaVersion)
	return fmt.Sprintf("%s_build_%s", active, sanitizedVersion)
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
	records, err := buildMeiliDocumentRecords(docs)
	if err != nil {
		return serrors.E(op, err)
	}
	_, err = e.submitRecords(ctx, op, records, true)
	return err
}

// UpsertAsync submits documents to Meilisearch without waiting for completion.
// Pending tasks are tracked and can be drained with WaitPending.
func (e *MeilisearchEngine) UpsertAsync(ctx context.Context, docs []SearchDocument) error {
	const op serrors.Op = "spotlight.MeilisearchEngine.UpsertAsync"

	if len(docs) == 0 {
		return nil
	}

	if err := e.setup(); err != nil {
		return serrors.E(op, err)
	}
	records, err := buildMeiliDocumentRecords(docs)
	if err != nil {
		return serrors.E(op, err)
	}
	taskUIDs, err := e.submitRecords(ctx, op, records, false)
	if err != nil {
		return err
	}

	e.pendingMu.Lock()
	e.pendingUIDs = append(e.pendingUIDs, taskUIDs...)
	e.pendingMu.Unlock()

	return nil
}

func buildMeiliDocumentRecords(docs []SearchDocument) ([]meiliDocumentRecord, error) {
	records := make([]meiliDocumentRecord, 0, len(docs))
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

		record["access_policy"] = doc.Access
		record["access_visibility"] = string(doc.Access.Visibility)
		record["owner_id"] = doc.Access.OwnerID
		record["allowed_users"] = doc.Access.AllowedUsers
		record["allowed_roles"] = doc.Access.AllowedRoles
		record["allowed_permissions"] = doc.Access.AllowedPermissions

		if len(doc.Embedding) > 0 {
			record["_vectors"] = map[string]interface{}{
				"default": doc.Embedding,
			}
		}

		payloadBytes, err := json.Marshal(record)
		if err != nil {
			return nil, err
		}

		records = append(records, meiliDocumentRecord{
			payload:   record,
			sizeBytes: len(payloadBytes),
			docID:     doc.ID,
		})
	}
	return records, nil
}

func chunkMeiliDocumentRecords(records []meiliDocumentRecord, maxDocs, maxPayloadBytes int) [][]meiliDocumentRecord {
	if len(records) == 0 {
		return nil
	}
	if maxDocs <= 0 {
		maxDocs = len(records)
	}
	if maxPayloadBytes <= 0 {
		maxPayloadBytes = int(^uint(0) >> 1)
	}

	batches := make([][]meiliDocumentRecord, 0, max(1, len(records)/maxDocs))
	current := make([]meiliDocumentRecord, 0, min(len(records), maxDocs))
	currentBytes := 2 // Opening and closing JSON array brackets.

	flush := func() {
		if len(current) == 0 {
			return
		}
		batches = append(batches, current)
		current = make([]meiliDocumentRecord, 0, min(len(records), maxDocs))
		currentBytes = 2
	}

	for _, record := range records {
		additionalBytes := record.sizeBytes
		if len(current) > 0 {
			additionalBytes++ // Comma separator in JSON array.
		}
		if len(current) > 0 && (len(current) >= maxDocs || currentBytes+additionalBytes > maxPayloadBytes) {
			flush()
		}
		current = append(current, record)
		currentBytes += additionalBytes
	}
	flush()

	return batches
}

func (e *MeilisearchEngine) submitRecords(
	ctx context.Context,
	op serrors.Op,
	records []meiliDocumentRecord,
	waitForTask bool,
) ([]int64, error) {
	var allTaskUIDs []int64
	for _, batch := range chunkMeiliDocumentRecords(records, meiliMaxDocumentsPerRequest, meiliSafePayloadLimitBytes) {
		taskUIDs, err := e.submitRecordBatch(ctx, op, batch, waitForTask)
		if err != nil {
			return nil, err
		}
		allTaskUIDs = append(allTaskUIDs, taskUIDs...)
	}
	return allTaskUIDs, nil
}

func (e *MeilisearchEngine) submitRecordBatch(
	ctx context.Context,
	op serrors.Op,
	batch []meiliDocumentRecord,
	waitForTask bool,
) ([]int64, error) {
	taskUIDs, err := e.trySubmitRecordBatch(ctx, batch, waitForTask)
	if err == nil {
		return taskUIDs, nil
	}
	if !isMeiliPayloadTooLarge(err) {
		return nil, serrors.E(op, err)
	}
	if len(batch) == 1 {
		return nil, serrors.E(op, fmt.Sprintf("single document %s exceeded Meilisearch payload limit", batch[0].docID), err)
	}

	mid := len(batch) / 2
	leftTaskUIDs, leftErr := e.submitRecordBatch(ctx, op, batch[:mid], waitForTask)
	if leftErr != nil {
		return nil, leftErr
	}
	rightTaskUIDs, rightErr := e.submitRecordBatch(ctx, op, batch[mid:], waitForTask)
	if rightErr != nil {
		return nil, rightErr
	}
	return append(leftTaskUIDs, rightTaskUIDs...), nil
}

func (e *MeilisearchEngine) trySubmitRecordBatch(
	ctx context.Context,
	batch []meiliDocumentRecord,
	waitForTask bool,
) ([]int64, error) {
	if len(batch) == 0 {
		return nil, nil
	}

	pk := "pk"
	task, err := e.client.Index(e.indexName).AddDocuments(meiliRecordPayloads(batch), &meilisearch.DocumentOptions{
		PrimaryKey: &pk,
	})
	if err != nil {
		return nil, err
	}
	if waitForTask {
		if _, err := e.client.WaitForTask(task.TaskUID, 100*time.Millisecond); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return []int64{task.TaskUID}, nil
}

func meiliRecordPayloads(records []meiliDocumentRecord) []map[string]interface{} {
	payloads := make([]map[string]interface{}, 0, len(records))
	for _, record := range records {
		payloads = append(payloads, record.payload)
	}
	return payloads
}

func isMeiliPayloadTooLarge(err error) bool {
	var meiliErr *meilisearch.Error
	if !errors.As(err, &meiliErr) {
		return false
	}
	return meiliErr.StatusCode == http.StatusRequestEntityTooLarge || meiliErr.MeilisearchApiError.Code == "payload_too_large"
}

// WaitPending blocks until all async upsert tasks have completed in Meilisearch.
func (e *MeilisearchEngine) WaitPending(ctx context.Context) error {
	const op serrors.Op = "spotlight.MeilisearchEngine.WaitPending"

	e.pendingMu.Lock()
	uids := e.pendingUIDs
	e.pendingUIDs = nil
	e.pendingMu.Unlock()

	for _, uid := range uids {
		if _, err := e.client.WaitForTask(uid, 250*time.Millisecond); err != nil {
			return serrors.E(op, err)
		}
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

func (e *MeilisearchEngine) DeleteTenant(ctx context.Context, tenantID uuid.UUID) error {
	const op serrors.Op = "spotlight.MeilisearchEngine.DeleteTenant"

	if tenantID == uuid.Nil {
		return nil
	}
	if err := e.setup(); err != nil {
		return serrors.E(op, err)
	}

	filter := fmt.Sprintf(`tenant_id = "%s"`, escapeFilterString(tenantID.String()))
	task, err := e.client.Index(e.indexName).DeleteDocumentsByFilterWithContext(ctx, filter, nil)
	if err != nil {
		return serrors.E(op, err)
	}
	if task != nil {
		if _, err := e.client.WaitForTask(task.TaskUID, 100*time.Millisecond); err != nil {
			return serrors.E(op, err)
		}
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

// Stats returns runtime statistics about the Meilisearch index.
func (e *MeilisearchEngine) Stats(ctx context.Context) (*IndexStats, error) {
	const op serrors.Op = "spotlight.MeilisearchEngine.Stats"

	state, err := e.inspectSearchIndex(e.activeName)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if !state.exists {
		return &IndexStats{}, nil
	}

	stats, err := e.client.Index(state.searchableName).GetStats()
	if err != nil {
		return nil, serrors.E(op, err)
	}

	result := &IndexStats{
		TotalDocuments: state.documents,
		SchemaVersion:  state.schemaVersion,
		IsSearchable:   state.searchable(),
	}
	if stats != nil {
		result.FieldDistribution = stats.FieldDistribution
	}

	// Get per-provider document counts via faceted search.
	providerCounts := make(map[string]int64)
	facetResp, facetErr := e.client.Index(state.searchableName).Search("", &meilisearch.SearchRequest{
		Facets: []string{"provider"},
		Limit:  0,
	})
	if facetErr == nil && facetResp != nil && len(facetResp.FacetDistribution) > 0 {
		var allFacets map[string]map[string]float64
		if jsonErr := json.Unmarshal(facetResp.FacetDistribution, &allFacets); jsonErr == nil {
			if pf, ok := allFacets["provider"]; ok {
				for k, v := range pf {
					providerCounts[k] = int64(v)
				}
			}
		}
	}
	result.ProviderDocumentCounts = providerCounts

	return result, nil
}

func (e *MeilisearchEngine) searchOnce(req SearchRequest, query, filter string, limit int) ([]SearchHit, error) {
	indexName, err := e.resolveSearchIndexName()
	if err != nil {
		return nil, err
	}
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
	resp, err := e.client.Index(indexName).Search(query, searchReq)
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

func spotlightIndexFieldsReady(fieldDistribution map[string]int64) bool {
	if len(fieldDistribution) == 0 {
		return false
	}
	requiredFields := []string{
		"domain",
		"description",
		"search_text",
		"exact_terms",
		"schema_version",
		"access_visibility",
		"owner_id",
		"allowed_users",
		"allowed_roles",
		"allowed_permissions",
	}
	for _, field := range requiredFields {
		if _, ok := fieldDistribution[field]; !ok {
			return false
		}
	}
	return true
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
