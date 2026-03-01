package subscription

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

func (s *service) CheckLimit(ctx context.Context, tenantID uuid.UUID, entityType string) (*LimitResult, error) {
	state, err := s.entitlementState(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	current, err := s.repo.GetEntityCount(ctx, tenantID, entityType)
	if err != nil {
		return nil, err
	}

	limit, ok := state.EntityLimits[entityType]
	if !ok {
		limit = -1
	}
	result := buildLimitResult(entityType, current, limit)
	return result, nil
}

func (s *service) GetLimits(ctx context.Context, tenantID uuid.UUID) (map[string]Limit, error) {
	state, err := s.entitlementState(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	counts, err := s.repo.GetEntityCounts(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	keys := make(map[string]struct{}, len(state.EntityLimits)+len(counts))
	for key := range state.EntityLimits {
		keys[key] = struct{}{}
	}
	for key := range counts {
		keys[key] = struct{}{}
	}

	result := make(map[string]Limit, len(keys))
	for key := range keys {
		current := counts[key]
		limit, ok := state.EntityLimits[key]
		if !ok {
			limit = -1
		}
		percentage := calculatePercentage(current, limit)
		result[key] = Limit{
			EntityType:  key,
			Current:     current,
			Max:         limit,
			IsUnlimited: limit < 0,
			Percentage:  percentage,
		}
	}

	return result, nil
}

func (s *service) IncrementCount(ctx context.Context, tenantID uuid.UUID, entityType string) error {
	result, err := s.CheckLimit(ctx, tenantID, entityType)
	if err != nil {
		return err
	}
	if !result.Allowed {
		return ErrLimitExceeded{EntityType: entityType, Current: result.Current, Limit: result.Limit}
	}
	return s.repo.IncrementEntityCount(ctx, tenantID, entityType)
}

func (s *service) DecrementCount(ctx context.Context, tenantID uuid.UUID, entityType string) error {
	return s.repo.DecrementEntityCount(ctx, tenantID, entityType)
}

func (s *service) SetCount(ctx context.Context, tenantID uuid.UUID, entityType string, count int) error {
	return s.repo.SetEntityCount(ctx, tenantID, entityType, count)
}

func (s *service) CheckSeatLimit(ctx context.Context, tenantID uuid.UUID) (*LimitResult, error) {
	state, err := s.entitlementState(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	entitlement, err := s.ensureEntitlement(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	limit := -1
	if state.SeatLimit != nil {
		limit = *state.SeatLimit
	}
	result := buildLimitResult("seats", entitlement.CurrentSeats, limit)
	return result, nil
}

func (s *service) AddSeat(ctx context.Context, tenantID uuid.UUID) error {
	state, err := s.entitlementState(ctx, tenantID)
	if err != nil {
		return err
	}
	if state.SeatLimit == nil || *state.SeatLimit < 0 {
		return s.repo.IncrementSeat(ctx, tenantID)
	}
	ok, err := s.repo.AddSeatIfBelow(ctx, tenantID, *state.SeatLimit)
	if err != nil {
		return err
	}
	if !ok {
		current, currentErr := s.repo.GetEntitlement(ctx, tenantID)
		if currentErr != nil {
			return ErrLimitExceeded{EntityType: "seats", Current: *state.SeatLimit, Limit: *state.SeatLimit}
		}
		return ErrLimitExceeded{EntityType: "seats", Current: current.CurrentSeats, Limit: *state.SeatLimit}
	}
	return nil
}

func (s *service) RemoveSeat(ctx context.Context, tenantID uuid.UUID) error {
	return s.repo.DecrementSeat(ctx, tenantID)
}

func buildLimitResult(entityType string, current, limit int) *LimitResult {
	if limit < 0 {
		return &LimitResult{
			Allowed:    true,
			Current:    current,
			Limit:      -1,
			Percentage: 0,
			Message:    fmt.Sprintf("%s is unlimited", entityType),
		}
	}
	percentage := calculatePercentage(current, limit)
	allowed := current < limit
	message := fmt.Sprintf("%s usage: %d/%d", entityType, current, limit)
	if !allowed {
		message = fmt.Sprintf("%s limit exceeded: %d/%d", entityType, current, limit)
	}
	return &LimitResult{
		Allowed:    allowed,
		Current:    current,
		Limit:      limit,
		Percentage: percentage,
		Message:    message,
	}
}

func calculatePercentage(current, limit int) float64 {
	if limit <= 0 {
		return 0
	}
	percentage := float64(current) / float64(limit)
	if percentage < 0 {
		return 0
	}
	if percentage > 1 {
		return 1
	}
	return percentage
}
