package services

import (
	"context"
	"time"

	"github.com/iota-uz/iota-sdk/modules/superadmin/domain"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain/entities"
	"github.com/pkg/errors"
)

// AnalyticsService provides business logic for analytics operations
type AnalyticsService struct {
	repo domain.AnalyticsQueryRepository
}

// NewAnalyticsService creates a new analytics service
func NewAnalyticsService(repo domain.AnalyticsQueryRepository) *AnalyticsService {
	return &AnalyticsService{
		repo: repo,
	}
}

// GetDashboardMetrics returns all dashboard metrics for the specified date range
// If startDate is zero, defaults to 30 days ago
// If endDate is zero, defaults to now
func (s *AnalyticsService) GetDashboardMetrics(ctx context.Context, startDate, endDate time.Time) (*entities.Analytics, error) {
	// Set default date range if not provided
	if startDate.IsZero() {
		startDate = time.Now().AddDate(0, 0, -30)
	}
	if endDate.IsZero() {
		endDate = time.Now()
	}

	// Validate date range
	if startDate.After(endDate) {
		return nil, errors.New("start date cannot be after end date")
	}

	// Retrieve metrics from repository
	metrics, err := s.repo.GetDashboardMetrics(ctx, startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get dashboard metrics")
	}

	// Fetch time series data for charts
	userSignups, err := s.repo.GetUserSignupsTimeSeries(ctx, startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user signups time series")
	}
	metrics.UserSignupsTimeSeries = userSignups

	tenantSignups, err := s.repo.GetTenantSignupsTimeSeries(ctx, startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant signups time series")
	}
	metrics.TenantSignupsTimeSeries = tenantSignups

	return metrics, nil
}
