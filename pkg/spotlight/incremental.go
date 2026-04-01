package spotlight

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

type ProjectionOp string

const (
	ProjectionOpCreate  ProjectionOp = "create"
	ProjectionOpUpdate  ProjectionOp = "update"
	ProjectionOpDelete  ProjectionOp = "delete"
	ProjectionOpReplace ProjectionOp = "replace"
)

type ProjectionEvent struct {
	TenantID      uuid.UUID
	ProviderID    string
	EntityType    string
	EntityID      string
	Operation     ProjectionOp
	Language      string
	OccurredAt    time.Time
	Version       uint64
	CorrelationID string
	Source        string
	Metadata      map[string]string
}

type IncrementalBinding struct {
	Name       string
	ProviderID string
	Match      func(ProjectionEvent) bool
	Refs       func(ProjectionEvent) []DocumentRef
}

type ResolvedBinding struct {
	Name       string
	ProviderID string
	Refs       []DocumentRef
}

type IncrementalBindingRegistry struct {
	mu       sync.RWMutex
	bindings []IncrementalBinding
}

func NewIncrementalBindingRegistry() *IncrementalBindingRegistry {
	return &IncrementalBindingRegistry{}
}

func (r *IncrementalBindingRegistry) Register(binding IncrementalBinding) {
	if binding.Name == "" || binding.ProviderID == "" {
		logrus.
			WithField("binding_name", binding.Name).
			WithField("binding_provider_id", binding.ProviderID).
			Warn("incremental binding registry: rejected binding with empty Name or ProviderID")
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bindings = append(r.bindings, binding)
	sort.SliceStable(r.bindings, func(i, j int) bool {
		if r.bindings[i].ProviderID != r.bindings[j].ProviderID {
			return r.bindings[i].ProviderID < r.bindings[j].ProviderID
		}
		return r.bindings[i].Name < r.bindings[j].Name
	})
}

func (r *IncrementalBindingRegistry) Resolve(event ProjectionEvent) []ResolvedBinding {
	r.mu.RLock()
	bindings := append([]IncrementalBinding(nil), r.bindings...)
	r.mu.RUnlock()

	out := make([]ResolvedBinding, 0, len(bindings))
	for _, binding := range bindings {
		if event.ProviderID != "" && binding.ProviderID != "" && binding.ProviderID != event.ProviderID {
			continue
		}
		if binding.Match != nil && !binding.Match(event) {
			continue
		}

		var refs []DocumentRef
		if binding.Refs != nil {
			refs = binding.Refs(event)
		}
		if len(refs) == 0 {
			continue
		}

		out = append(out, ResolvedBinding{
			Name:       binding.Name,
			ProviderID: binding.ProviderID,
			Refs:       dedupeRefs(refs),
		})
	}
	return out
}

type IncrementalProvider interface {
	SearchProvider
	LoadDocuments(ctx context.Context, scope ProviderScope, refs []DocumentRef, emit DocumentBatchEmitter) error
}

type IncrementalLoader func(ctx context.Context, scope ProviderScope, refs []DocumentRef, emit DocumentBatchEmitter) error

type incrementalProviderAdapter struct {
	SearchProvider
	load IncrementalLoader
}

func NewIncrementalProviderAdapter(provider SearchProvider, load IncrementalLoader) IncrementalProvider {
	if provider == nil || load == nil {
		return nil
	}
	return &incrementalProviderAdapter{
		SearchProvider: provider,
		load:           load,
	}
}

func (p *incrementalProviderAdapter) LoadDocuments(
	ctx context.Context,
	scope ProviderScope,
	refs []DocumentRef,
	emit DocumentBatchEmitter,
) error {
	return p.load(ctx, scope, refs, emit)
}

type IncrementalSyncer struct {
	engine   IndexEngine
	logger   *logrus.Entry
	tenantID uuid.UUID
}

func NewIncrementalSyncer(
	engine IndexEngine,
	tenantID uuid.UUID,
	logger *logrus.Entry,
) *IncrementalSyncer {
	if logger == nil {
		logger = logrus.NewEntry(logrus.StandardLogger())
	}
	return &IncrementalSyncer{
		engine:   engine,
		logger:   logger,
		tenantID: tenantID,
	}
}

func (s *IncrementalSyncer) SyncDocuments(
	ctx context.Context,
	projector string,
	docIDs []string,
	docs []SearchDocument,
) error {
	const op serrors.Op = "spotlight.IncrementalSyncer.SyncDocuments"

	if s == nil || s.engine == nil {
		return serrors.E(op, "spotlight engine is not configured")
	}
	docIDs = dedupeRefIDs(docIDs)
	if len(docIDs) == 0 && len(docs) == 0 {
		return nil
	}

	found := make(map[string]struct{}, len(docs))
	for i := range docs {
		if docs[i].TenantID == uuid.Nil {
			docs[i].TenantID = s.tenantID
		}
		found[docs[i].ID] = struct{}{}
	}

	if len(docs) > 0 {
		if err := s.engine.Upsert(ctx, docs); err != nil {
			return serrors.E(op, err)
		}
	}

	deletes := make([]DocumentRef, 0, len(docIDs))
	for _, docID := range docIDs {
		if _, ok := found[docID]; ok {
			continue
		}
		deletes = append(deletes, DocumentRef{
			TenantID: s.tenantID,
			ID:       docID,
		})
	}

	if len(deletes) > 0 {
		if err := s.engine.Delete(ctx, deletes); err != nil {
			return serrors.E(op, err)
		}
	}

	if s.logger != nil {
		s.logger.WithFields(logrus.Fields{
			"projector": projector,
			"tenant_id": s.tenantID.String(),
			"upserts":   len(docs),
			"deletes":   len(deletes),
		}).Info("spotlight incremental sync completed")
	}

	return nil
}

func (s *IncrementalSyncer) SyncStream(
	ctx context.Context,
	projector string,
	scope ProviderScope,
	docIDs []string,
	stream func(DocumentBatchEmitter) error,
) error {
	const op serrors.Op = "spotlight.IncrementalSyncer.SyncStream"

	if s == nil || s.engine == nil {
		return serrors.E(op, "spotlight engine is not configured")
	}
	if stream == nil {
		return serrors.E(op, "spotlight stream is not configured")
	}

	docs, err := CollectDocumentStream(ctx, stream)
	if err != nil {
		return serrors.E(op, err)
	}
	for i := range docs {
		if docs[i].TenantID == uuid.Nil {
			docs[i].TenantID = scope.TenantID
		}
		if docs[i].Language == "" {
			docs[i].Language = scope.Language
		}
	}
	return s.SyncDocuments(ctx, projector, docIDs, docs)
}

func (s *IncrementalSyncer) SyncProviderRefs(
	ctx context.Context,
	projector string,
	provider IncrementalProvider,
	scope ProviderScope,
	refs []DocumentRef,
) error {
	const op serrors.Op = "spotlight.IncrementalSyncer.SyncProviderRefs"

	if provider == nil {
		return serrors.E(op, "incremental provider is not configured")
	}
	if err := s.SyncStream(ctx, projector, scope, refIDs(refs), func(emit DocumentBatchEmitter) error {
		return provider.LoadDocuments(ctx, scope, refs, emit)
	}); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func refIDs(refs []DocumentRef) []string {
	docIDs := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.ID == "" {
			continue
		}
		docIDs = append(docIDs, ref.ID)
	}
	return dedupeRefIDs(docIDs)
}

func dedupeRefIDs(docIDs []string) []string {
	seen := make(map[string]struct{}, len(docIDs))
	out := make([]string, 0, len(docIDs))
	for _, docID := range docIDs {
		if docID == "" {
			continue
		}
		if _, ok := seen[docID]; ok {
			continue
		}
		seen[docID] = struct{}{}
		out = append(out, docID)
	}
	return out
}

func dedupeRefs(refs []DocumentRef) []DocumentRef {
	seen := make(map[string]struct{}, len(refs))
	out := make([]DocumentRef, 0, len(refs))
	for _, ref := range refs {
		if ref.ID == "" {
			continue
		}
		key := ref.TenantID.String() + ":" + ref.ID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ref)
	}
	return out
}
