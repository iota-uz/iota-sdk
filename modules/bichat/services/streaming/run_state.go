package streaming

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type RunStateStore interface {
	CreateRun(ctx context.Context, run domain.GenerationRun) error
	GetActiveRunBySession(ctx context.Context, tenantID uuid.UUID, sessionID uuid.UUID) (domain.GenerationRun, error)
	UpdateRunSnapshot(ctx context.Context, tenantID, sessionID, runID uuid.UUID, partialContent string, partialMetadata map[string]any) error
	CompleteRun(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error
	CancelRun(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error
}

type RunStateManager struct {
	store RunStateStore
}

func NewRunStateManager(store RunStateStore) *RunStateManager {
	return &RunStateManager{store: store}
}

func (m *RunStateManager) CreateRunState(ctx context.Context, run domain.GenerationRun) (bool, error) {
	if m == nil || m.store == nil {
		return false, nil
	}
	if err := m.store.CreateRun(ctx, run); err != nil {
		return false, err
	}
	return true, nil
}

func (m *RunStateManager) GetPersistedRun(ctx context.Context, sessionID uuid.UUID) (domain.GenerationRun, error) {
	const op serrors.Op = "runStateManager.GetPersistedRun"
	if m == nil || m.store == nil {
		return nil, domain.ErrNoActiveRun
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return m.store.GetActiveRunBySession(ctx, tenantID, sessionID)
}

func (m *RunStateManager) UpdateRunSnapshot(ctx context.Context, tenantID, sessionID, runID uuid.UUID, partialContent string, partialMetadata map[string]any) error {
	if m == nil || m.store == nil {
		return nil
	}
	return m.store.UpdateRunSnapshot(ctx, tenantID, sessionID, runID, partialContent, partialMetadata)
}

func (m *RunStateManager) CompleteRunState(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error {
	if m == nil || m.store == nil {
		return nil
	}
	return m.store.CompleteRun(ctx, tenantID, sessionID, runID)
}

func (m *RunStateManager) CancelRunState(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error {
	if m == nil || m.store == nil {
		return nil
	}
	return m.store.CancelRun(ctx, tenantID, sessionID, runID)
}
