package backtest

import (
	"context"

	"github.com/google/uuid"
)

// ClaimBacktest claims the next due backtest as a worker does and returns its id and lease.
func (s *Svc) ClaimBacktest(ctx context.Context) (uuid.UUID, uuid.UUID, bool, error) {
	j, ok, err := s.claimBacktest(ctx)
	return j.id, j.lease, ok, err
}

// StillHolds is whether a live example of the backtest would run under lease.
func (s *Svc) StillHolds(ctx context.Context, tenant, id, lease uuid.UUID) (bool, error) {
	_, err := s.runExample(ctx, Actor{}, backtestJob{tenant: tenant, id: id, lease: lease, decisions: DecisionsLive}, nil, nil, "", Example{})
	if err == errCancelled {
		return false, nil
	}
	return true, nil
}
