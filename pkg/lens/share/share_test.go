package share

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSMTPMessageContainsWorkbookAttachment(t *testing.T) {
	t.Parallel()
	message, err := buildSMTPMessage("reports@example.com", Mail{
		Recipients: []string{"analyst@example.com"}, Subject: "Claims — weekly", Body: "Attached.",
		Attachment: Workbook{Filename: "claims.xlsx", Content: []byte("xlsx")},
	})
	require.NoError(t, err)
	text := string(message)
	require.Contains(t, text, "multipart/mixed")
	require.Contains(t, text, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	require.Contains(t, text, base64.StdEncoding.EncodeToString([]byte("xlsx")))
}

func TestMemoryStoreScopesViewsAndEnforcesRoleDefaults(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore()
	tenant := uuid.New()
	otherTenant := uuid.New()
	role := uint(7)
	owner := Access{TenantID: tenant, UserID: 11, RoleIDs: []uint{role}, Roles: []RoleRef{{ID: role, Name: "Analyst"}}, ManageTeam: true}
	personal, err := store.PutView(t.Context(), owner, SavedView{
		DashboardID: "claims", Name: "Mine", Scope: ViewScopePersonal, StateURL: "/claims?year=2026",
	})
	require.NoError(t, err)
	team, err := store.PutView(t.Context(), owner, SavedView{
		DashboardID: "claims", Name: "Team default", Scope: ViewScopeTeam, StateURL: "/claims?year=2025", DefaultRoleID: &role,
	})
	require.NoError(t, err)
	replacement, err := store.PutView(t.Context(), owner, SavedView{
		DashboardID: "claims", Name: "New team default", Scope: ViewScopeTeam, StateURL: "/claims?year=2024", DefaultRoleID: &role,
	})
	require.NoError(t, err)

	colleague := Access{TenantID: tenant, UserID: 12, RoleIDs: []uint{role}}
	views, err := store.ListViews(t.Context(), colleague, "claims")
	require.NoError(t, err)
	require.Len(t, views, 2)
	require.Equal(t, replacement.ID, views[0].ID)
	for _, view := range views {
		if view.ID == team.ID {
			require.Nil(t, view.DefaultRoleID)
		}
	}

	views, err = store.ListViews(t.Context(), Access{TenantID: otherTenant, UserID: 11}, "claims")
	require.NoError(t, err)
	require.Empty(t, views)
	require.ErrorIs(t, store.DeleteView(t.Context(), colleague, personal.ID), ErrNotFound)
	require.ErrorIs(t, store.DeleteView(t.Context(), Access{TenantID: tenant, UserID: 12, ManageTeam: true}, personal.ID), ErrNotFound)
}

func TestMemoryStoreRejectsUnassignableRoleAndScheduleAccessWithoutPermission(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore()
	tenant := uuid.New()
	role := uint(7)
	manager := Access{TenantID: tenant, UserID: 11, ManageTeam: true, Roles: []RoleRef{{ID: role, Name: "Analyst"}}}
	unknownRole := uint(99)
	_, err := store.PutView(t.Context(), manager, SavedView{
		DashboardID: "claims", Name: "Unknown role", Scope: ViewScopeTeam, StateURL: "/claims", DefaultRoleID: &unknownRole,
	})
	require.ErrorIs(t, err, ErrForbidden)

	view, err := store.PutView(t.Context(), manager, SavedView{
		DashboardID: "claims", Name: "Mine", Scope: ViewScopePersonal, StateURL: "/claims",
	})
	require.NoError(t, err)
	scheduler := Access{TenantID: tenant, UserID: 11, ScheduleMail: true}
	schedule, err := store.PutSchedule(t.Context(), scheduler, ExportSchedule{
		DashboardID: "claims", ViewID: view.ID, Name: "Weekly", Cron: "0 8 * * 1", Timezone: "UTC",
		Recipients: []string{"analyst@example.com"}, Enabled: true,
	})
	require.NoError(t, err)
	withoutPermission := Access{TenantID: tenant, UserID: 11}
	_, err = store.ListSchedules(t.Context(), withoutPermission, "claims")
	require.ErrorIs(t, err, ErrForbidden)
	require.ErrorIs(t, store.DeleteSchedule(t.Context(), withoutPermission, schedule.ID), ErrForbidden)
}

func TestTeamViewDowngradeRevokesOtherOwnersSchedules(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore()
	tenantID := uuid.New()
	manager := Access{TenantID: tenantID, UserID: 11, ManageTeam: true}
	view, err := store.PutView(t.Context(), manager, SavedView{
		DashboardID: "claims", Name: "Team claims", Scope: ViewScopeTeam, StateURL: "/claims?year=2026",
	})
	require.NoError(t, err)
	colleague := Access{TenantID: tenantID, UserID: 12, ScheduleMail: true}
	_, err = store.PutSchedule(t.Context(), colleague, ExportSchedule{
		DashboardID: "claims", ViewID: view.ID, Name: "Weekly", Cron: "0 8 * * 1", Timezone: "UTC",
		Recipients: []string{"colleague@example.com"}, Enabled: true,
	})
	require.NoError(t, err)

	view.Scope = ViewScopePersonal
	view.DefaultRoleID = nil
	_, err = store.PutView(t.Context(), manager, view)
	require.NoError(t, err)
	schedules, err := store.ListSchedules(t.Context(), colleague, "claims")
	require.NoError(t, err)
	require.Empty(t, schedules)
}

type recordingExporter struct {
	calls    int
	stateURL string
}

func (e *recordingExporter) ExportSavedView(_ context.Context, schedule ExportSchedule) (Workbook, error) {
	e.calls++
	e.stateURL = schedule.StateURL
	return Workbook{Filename: "claims.xlsx", Content: []byte("xlsx")}, nil
}

type recordingMailer struct{ mails []Mail }

func (m *recordingMailer) Send(_ context.Context, mail Mail) error {
	m.mails = append(m.mails, mail)
	return nil
}

type cancelingExporter struct{ cancel context.CancelFunc }

func (e cancelingExporter) ExportSavedView(context.Context, ExportSchedule) (Workbook, error) {
	e.cancel()
	return Workbook{Filename: "claims.xlsx", Content: []byte("xlsx")}, nil
}

type contextCheckingFinishStore struct {
	Store
	finishContextErr error
}

func (s *contextCheckingFinishStore) FinishSchedule(
	ctx context.Context,
	tenantID uuid.UUID,
	id uuid.UUID,
	claimedUntil time.Time,
	ranAt time.Time,
	nextRun time.Time,
	runErr error,
) error {
	s.finishContextErr = ctx.Err()
	if s.finishContextErr != nil {
		return s.finishContextErr
	}
	return s.Store.FinishSchedule(ctx, tenantID, id, claimedUntil, ranAt, nextRun, runErr)
}

func TestScheduleRunnerExportsAndMailsTheSavedSlice(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore()
	access := Access{TenantID: uuid.New(), UserID: 11, ScheduleMail: true}
	view, err := store.PutView(t.Context(), access, SavedView{
		DashboardID: "claims", Name: "Current claims", Scope: ViewScopePersonal, StateURL: "/claims?year=2026&_hidden=paid",
	})
	require.NoError(t, err)
	schedule, err := store.PutSchedule(t.Context(), access, ExportSchedule{
		DashboardID: "claims", ViewID: view.ID, Name: "Weekly claims", Cron: "0 8 * * 1", Timezone: "Asia/Tashkent",
		Recipients: []string{"analyst@example.com"}, Enabled: true,
	})
	require.NoError(t, err)
	dueAt := schedule.NextRunAt

	exporter := &recordingExporter{}
	mailer := &recordingMailer{}
	runner, err := NewScheduleRunner(store, exporter, mailer, 10)
	require.NoError(t, err)
	require.NoError(t, runner.RunDue(t.Context(), dueAt))
	require.Equal(t, 1, exporter.calls)
	require.Equal(t, "/claims?year=2026&_hidden=paid", exporter.stateURL)
	require.Len(t, mailer.mails, 1)
	require.Equal(t, "claims.xlsx", mailer.mails[0].Attachment.Filename)

	schedules, err := store.ListSchedules(t.Context(), access, "claims")
	require.NoError(t, err)
	require.True(t, schedules[0].LastRunAt.Equal(dueAt))
	require.True(t, schedules[0].NextRunAt.After(dueAt))
}

func TestScheduleRunnerFinalizesClaimAfterDeliveryContextCancellation(t *testing.T) {
	t.Parallel()
	memory := NewMemoryStore()
	store := &contextCheckingFinishStore{Store: memory}
	access := Access{TenantID: uuid.New(), UserID: 11, ScheduleMail: true}
	view, err := memory.PutView(t.Context(), access, SavedView{
		DashboardID: "claims", Name: "Current claims", Scope: ViewScopePersonal, StateURL: "/claims",
	})
	require.NoError(t, err)
	schedule, err := memory.PutSchedule(t.Context(), access, ExportSchedule{
		DashboardID: "claims", ViewID: view.ID, Name: "Weekly claims", Cron: "0 8 * * 1", Timezone: "UTC",
		Recipients: []string{"analyst@example.com"}, Enabled: true,
	})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	runner, err := NewScheduleRunner(store, cancelingExporter{cancel: cancel}, &recordingMailer{}, 1)
	require.NoError(t, err)
	require.NoError(t, runner.RunDue(ctx, schedule.NextRunAt))
	require.NoError(t, store.finishContextErr)
}

func TestMemoryScheduleFollowsTheExactUpdatedSavedSlice(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore()
	access := Access{TenantID: uuid.New(), UserID: 11, ScheduleMail: true}
	view, err := store.PutView(t.Context(), access, SavedView{
		DashboardID: "claims", Name: "Current claims", Scope: ViewScopePersonal, StateURL: "/claims?year=2025",
	})
	require.NoError(t, err)
	schedule, err := store.PutSchedule(t.Context(), access, ExportSchedule{
		DashboardID: "claims", ViewID: view.ID, Name: "Weekly claims", Cron: "0 8 * * 1", Timezone: "UTC",
		Recipients: []string{"analyst@example.com"}, Enabled: true,
	})
	require.NoError(t, err)
	view.StateURL = "/claims?year=2026&_f=status%3Apaid#losses"
	_, err = store.PutView(t.Context(), access, view)
	require.NoError(t, err)
	view.DashboardID = "other-dashboard"
	_, err = store.PutView(t.Context(), access, view)
	require.ErrorIs(t, err, ErrInvalid)
	view.DashboardID = "claims"
	schedule.DashboardID = "other-dashboard"
	_, err = store.PutSchedule(t.Context(), access, schedule)
	require.ErrorIs(t, err, ErrInvalid)

	due, err := store.DueSchedules(t.Context(), schedule.NextRunAt, 1)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Equal(t, view.StateURL, due[0].StateURL)
}

func TestScheduleFinishRequiresMatchingTenantAndActiveClaim(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore()
	access := Access{TenantID: uuid.New(), UserID: 11, ScheduleMail: true}
	view, err := store.PutView(t.Context(), access, SavedView{
		DashboardID: "claims", Name: "Current claims", Scope: ViewScopePersonal, StateURL: "/claims",
	})
	require.NoError(t, err)
	schedule, err := store.PutSchedule(t.Context(), access, ExportSchedule{
		DashboardID: "claims", ViewID: view.ID, Name: "Weekly claims", Cron: "0 8 * * 1", Timezone: "UTC",
		Recipients: []string{"analyst@example.com"}, Enabled: true,
	})
	require.NoError(t, err)
	due, err := store.DueSchedules(t.Context(), schedule.NextRunAt, 1)
	require.NoError(t, err)
	require.Len(t, due, 1)

	next := due[0].NextRunAt.Add(time.Hour)
	require.ErrorIs(t, store.FinishSchedule(
		t.Context(), uuid.New(), due[0].ID, due[0].NextRunAt, schedule.NextRunAt, next, nil,
	), ErrClaimLost)
	require.ErrorIs(t, store.FinishSchedule(
		t.Context(), access.TenantID, due[0].ID, due[0].NextRunAt.Add(time.Second), schedule.NextRunAt, next, nil,
	), ErrClaimLost)
	require.NoError(t, store.FinishSchedule(
		t.Context(), access.TenantID, due[0].ID, due[0].NextRunAt, schedule.NextRunAt, next, nil,
	))
	require.ErrorIs(t, store.FinishSchedule(
		t.Context(), access.TenantID, due[0].ID, due[0].NextRunAt, schedule.NextRunAt, next, nil,
	), ErrClaimLost)
}

func TestScheduleValidationRejectsInvalidStateAndRecipients(t *testing.T) {
	t.Parallel()
	view := SavedView{TenantID: uuid.New(), OwnerUserID: 1, DashboardID: "claims", Name: "Bad", Scope: ViewScopePersonal, StateURL: "https://evil.example/claims"}
	require.ErrorContains(t, view.Validate(), "site-relative")
	view.StateURL = `/\attacker.example/claims`
	require.ErrorContains(t, view.Validate(), "site-relative")
	schedule := ExportSchedule{
		TenantID: uuid.New(), OwnerUserID: 1, DashboardID: "claims", ViewID: uuid.New(), Name: "Bad",
		Cron: "not cron", Timezone: "UTC", Recipients: []string{"not-an-email"}, Enabled: true,
	}
	require.Error(t, schedule.Validate())
	_, err := schedule.Next(time.Now())
	require.Error(t, err)
}

func TestSMTPRejectsHeaderInjectionAndWrapsWorkbookEncoding(t *testing.T) {
	t.Parallel()
	_, err := NewSMTPMailer("smtp.example.com", 587, "", "", "reports@example.com\r\nBcc: attacker@example.com")
	require.Error(t, err)
	_, err = buildSMTPMessage("reports@example.com", Mail{
		Recipients: []string{"analyst@example.com\r\nBcc: attacker@example.com"}, Subject: "Report",
		Attachment: Workbook{Filename: "report.xlsx", Content: []byte("xlsx")},
	})
	require.Error(t, err)

	message, err := buildSMTPMessage("reports@example.com", Mail{
		Recipients: []string{"analyst@example.com"}, Subject: "Report\r\nBcc: attacker@example.com",
		Attachment: Workbook{Filename: "report.xlsx", Content: make([]byte, 4_096)},
	})
	require.NoError(t, err)
	text := string(message)
	require.NotContains(t, text, "\r\nBcc:")
	for _, line := range strings.Split(text, "\r\n") {
		require.LessOrEqual(t, len(line), 998, "SMTP line exceeds RFC limit")
	}
}
