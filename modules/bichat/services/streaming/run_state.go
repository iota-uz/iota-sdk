// Package streaming provides this package.
package streaming

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type RunStateStore interface {
	CreateRun(ctx context.Context, run domain.GenerationRun) error
	GetActiveRunBySession(ctx context.Context, tenantID uuid.UUID, sessionID uuid.UUID) (domain.GenerationRun, error)
	GetRunByID(ctx context.Context, tenantID uuid.UUID, runID uuid.UUID) (domain.GenerationRun, error)
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
	const op serrors.Op = "runStateManager.CreateRunState"
	if m == nil || m.store == nil {
		return false, nil
	}
	if err := m.store.CreateRun(ctx, run); err != nil {
		return false, serrors.E(op, err)
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
		if errors.Is(err, composables.ErrNoTenantIDFound) {
			return nil, domain.ErrNoActiveRun
		}
		return nil, serrors.E(op, err)
	}
	return m.store.GetActiveRunBySession(ctx, tenantID, sessionID)
}

func (m *RunStateManager) GetPersistedRunByID(ctx context.Context, runID uuid.UUID) (domain.GenerationRun, error) {
	const op serrors.Op = "runStateManager.GetPersistedRunByID"
	if m == nil || m.store == nil {
		return nil, domain.ErrRunNotFound
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		if errors.Is(err, composables.ErrNoTenantIDFound) {
			return nil, domain.ErrRunNotFound
		}
		return nil, serrors.E(op, err)
	}
	return m.store.GetRunByID(ctx, tenantID, runID)
}

func (m *RunStateManager) UpdateRunSnapshot(ctx context.Context, tenantID, sessionID, runID uuid.UUID, partialContent string, partialMetadata map[string]any) error {
	const op serrors.Op = "runStateManager.UpdateRunSnapshot"
	if m == nil || m.store == nil {
		return nil
	}
	if err := m.store.UpdateRunSnapshot(ctx, tenantID, sessionID, runID, partialContent, partialMetadata); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (m *RunStateManager) CompleteRunState(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error {
	const op serrors.Op = "runStateManager.CompleteRunState"
	if m == nil || m.store == nil {
		return nil
	}
	if err := m.store.CompleteRun(ctx, tenantID, sessionID, runID); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (m *RunStateManager) CancelRunState(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error {
	const op serrors.Op = "runStateManager.CancelRunState"
	if m == nil || m.store == nil {
		return nil
	}
	if err := m.store.CancelRun(ctx, tenantID, sessionID, runID); err != nil {
		return serrors.E(op, err)
	}
	return nil
}
