package share

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Store interface {
	ListViews(context.Context, Access, string) ([]SavedView, error)
	PutView(context.Context, Access, SavedView) (SavedView, error)
	DeleteView(context.Context, Access, uuid.UUID) error
	ListSchedules(context.Context, Access, string) ([]ExportSchedule, error)
	PutSchedule(context.Context, Access, ExportSchedule) (ExportSchedule, error)
	DeleteSchedule(context.Context, Access, uuid.UUID) error
	DueSchedules(context.Context, time.Time, int) ([]ExportSchedule, error)
	FinishSchedule(context.Context, uuid.UUID, uuid.UUID, time.Time, time.Time, time.Time, error) error
}

type MemoryStore struct {
	mu        sync.Mutex
	views     map[uuid.UUID]SavedView
	schedules map[uuid.UUID]ExportSchedule
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{views: make(map[uuid.UUID]SavedView), schedules: make(map[uuid.UUID]ExportSchedule)}
}

func (s *MemoryStore) ListViews(_ context.Context, access Access, dashboardID string) ([]SavedView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	views := make([]SavedView, 0)
	for _, view := range s.views {
		if view.TenantID != access.TenantID || view.DashboardID != dashboardID {
			continue
		}
		if view.Scope == ViewScopePersonal && view.OwnerUserID != access.UserID {
			continue
		}
		views = append(views, view)
	}
	slices.SortFunc(views, func(a, b SavedView) int {
		switch {
		case a.DefaultRoleID != nil && b.DefaultRoleID == nil:
			return -1
		case a.DefaultRoleID == nil && b.DefaultRoleID != nil:
			return 1
		case a.DefaultRoleID != nil && b.DefaultRoleID != nil && *a.DefaultRoleID != *b.DefaultRoleID:
			return cmp.Compare(*a.DefaultRoleID, *b.DefaultRoleID)
		case !strings.EqualFold(a.Name, b.Name):
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		}
		return strings.Compare(a.ID.String(), b.ID.String())
	})
	return views, nil
}

func (s *MemoryStore) PutView(_ context.Context, access Access, view SavedView) (SavedView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	view.TenantID = access.TenantID
	view.OwnerUserID = access.UserID
	if view.Scope == ViewScopeTeam && !access.ManageTeam {
		return SavedView{}, ErrForbidden
	}
	if view.DefaultRoleID != nil && !access.ManageTeam {
		return SavedView{}, ErrForbidden
	}
	if view.DefaultRoleID != nil && !canAssignDefaultRole(access, *view.DefaultRoleID) {
		return SavedView{}, ErrForbidden
	}
	if err := view.Validate(); err != nil {
		return SavedView{}, invalidInput(err)
	}
	now := time.Now().UTC()
	if view.ID == uuid.Nil {
		view.ID = uuid.New()
		view.OwnerUserID = access.UserID
		view.CreatedAt = now
	} else if existing, ok := s.views[view.ID]; ok {
		if existing.TenantID != access.TenantID ||
			existing.OwnerUserID != access.UserID && (existing.Scope != ViewScopeTeam || !access.ManageTeam) {
			return SavedView{}, ErrNotFound
		}
		if existing.DashboardID != view.DashboardID {
			return SavedView{}, invalidInput(errors.New("saved view dashboard cannot be changed"))
		}
		view.OwnerUserID = existing.OwnerUserID
		view.CreatedAt = existing.CreatedAt
	} else {
		return SavedView{}, ErrNotFound
	}
	if view.DefaultRoleID != nil {
		for id, candidate := range s.views {
			if candidate.TenantID == view.TenantID && candidate.DashboardID == view.DashboardID && candidate.DefaultRoleID != nil && *candidate.DefaultRoleID == *view.DefaultRoleID {
				candidate.DefaultRoleID = nil
				candidate.UpdatedAt = now
				s.views[id] = candidate
			}
		}
	}
	view.UpdatedAt = now
	s.views[view.ID] = view
	for id, schedule := range s.schedules {
		if schedule.TenantID != view.TenantID || schedule.DashboardID != view.DashboardID || schedule.ViewID != view.ID {
			continue
		}
		if view.Scope == ViewScopePersonal && schedule.OwnerUserID != view.OwnerUserID {
			delete(s.schedules, id)
			continue
		}
		schedule.StateURL = view.StateURL
		s.schedules[id] = schedule
	}
	return view, nil
}

func (s *MemoryStore) DeleteView(_ context.Context, access Access, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	view, ok := s.views[id]
	if !ok || view.TenantID != access.TenantID ||
		view.OwnerUserID != access.UserID && (view.Scope != ViewScopeTeam || !access.ManageTeam) {
		return ErrNotFound
	}
	delete(s.views, id)
	for scheduleID, schedule := range s.schedules {
		if schedule.ViewID == id {
			delete(s.schedules, scheduleID)
		}
	}
	return nil
}

func (s *MemoryStore) ListSchedules(_ context.Context, access Access, dashboardID string) ([]ExportSchedule, error) {
	if !access.ScheduleMail {
		return nil, ErrForbidden
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	schedules := make([]ExportSchedule, 0)
	for _, schedule := range s.schedules {
		if schedule.TenantID != access.TenantID || schedule.OwnerUserID != access.UserID || schedule.DashboardID != dashboardID {
			continue
		}
		view, ok := s.views[schedule.ViewID]
		if !ok || view.TenantID != schedule.TenantID || view.DashboardID != schedule.DashboardID ||
			view.Scope == ViewScopePersonal && view.OwnerUserID != schedule.OwnerUserID {
			continue
		}
		schedule.StateURL = view.StateURL
		schedules = append(schedules, schedule)
	}
	slices.SortFunc(schedules, func(a, b ExportSchedule) int { return a.NextRunAt.Compare(b.NextRunAt) })
	return schedules, nil
}

func (s *MemoryStore) PutSchedule(_ context.Context, access Access, schedule ExportSchedule) (ExportSchedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !access.ScheduleMail {
		return ExportSchedule{}, ErrForbidden
	}
	schedule.TenantID = access.TenantID
	schedule.OwnerUserID = access.UserID
	var existingCreatedAt time.Time
	if schedule.ID != uuid.Nil {
		existing, exists := s.schedules[schedule.ID]
		if !exists || existing.TenantID != access.TenantID || existing.OwnerUserID != access.UserID {
			return ExportSchedule{}, ErrNotFound
		}
		if existing.DashboardID != schedule.DashboardID {
			return ExportSchedule{}, invalidInput(errors.New("schedule dashboard cannot be changed"))
		}
		existingCreatedAt = existing.CreatedAt
	}
	view, ok := s.views[schedule.ViewID]
	if !ok || view.TenantID != access.TenantID || view.DashboardID != schedule.DashboardID || (view.Scope == ViewScopePersonal && view.OwnerUserID != access.UserID) {
		return ExportSchedule{}, ErrNotFound
	}
	schedule.StateURL = view.StateURL
	if err := schedule.Validate(); err != nil {
		return ExportSchedule{}, invalidInput(err)
	}
	now := time.Now().UTC()
	if schedule.ID == uuid.Nil {
		schedule.ID = uuid.New()
		schedule.CreatedAt = now
	} else if !existingCreatedAt.IsZero() {
		schedule.CreatedAt = existingCreatedAt
	}
	if schedule.Enabled {
		next, err := schedule.Next(now)
		if err != nil {
			return ExportSchedule{}, invalidInput(err)
		}
		schedule.NextRunAt = next
	} else {
		schedule.NextRunAt = time.Time{}
	}
	schedule.UpdatedAt = now
	s.schedules[schedule.ID] = schedule
	return schedule, nil
}

func (s *MemoryStore) DeleteSchedule(_ context.Context, access Access, id uuid.UUID) error {
	if !access.ScheduleMail {
		return ErrForbidden
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	schedule, ok := s.schedules[id]
	if !ok || schedule.TenantID != access.TenantID || schedule.OwnerUserID != access.UserID {
		return ErrNotFound
	}
	delete(s.schedules, id)
	return nil
}

func (s *MemoryStore) DueSchedules(_ context.Context, now time.Time, limit int) ([]ExportSchedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		return nil, nil
	}
	due := make([]ExportSchedule, 0, limit)
	for _, schedule := range s.schedules {
		if !schedule.Enabled || schedule.NextRunAt.After(now) {
			continue
		}
		view, ok := s.views[schedule.ViewID]
		if !ok || view.TenantID != schedule.TenantID || view.DashboardID != schedule.DashboardID ||
			view.Scope == ViewScopePersonal && view.OwnerUserID != schedule.OwnerUserID {
			continue
		}
		schedule.StateURL = view.StateURL
		due = append(due, schedule)
	}
	slices.SortFunc(due, func(a, b ExportSchedule) int { return a.NextRunAt.Compare(b.NextRunAt) })
	if len(due) > limit {
		due = due[:limit]
	}
	claimedUntil := now.UTC().Add(scheduleClaimLease)
	for i := range due {
		schedule := due[i]
		schedule.NextRunAt = claimedUntil
		schedule.UpdatedAt = now.UTC()
		s.schedules[schedule.ID] = schedule
		due[i] = schedule
	}
	return due, nil
}

func (s *MemoryStore) FinishSchedule(
	_ context.Context,
	tenantID uuid.UUID,
	id uuid.UUID,
	claimedUntil time.Time,
	ranAt time.Time,
	nextRun time.Time,
	runErr error,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	schedule, ok := s.schedules[id]
	if !ok || schedule.TenantID != tenantID || !schedule.NextRunAt.Equal(claimedUntil) {
		return ErrClaimLost
	}
	schedule.LastRunAt = ranAt.UTC()
	schedule.NextRunAt = nextRun.UTC()
	schedule.LastError = ""
	if runErr != nil {
		schedule.LastError = runErr.Error()
	}
	schedule.UpdatedAt = time.Now().UTC()
	s.schedules[id] = schedule
	return nil
}
