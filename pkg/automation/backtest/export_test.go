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
	err := s.holds(ctx, s.cfg.DB, backtestJob{tenant: tenant, id: id, lease: lease}, false)
	if err == errCancelled {
		return false, nil
	}
	return err == nil, err
}
