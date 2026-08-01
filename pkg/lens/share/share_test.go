package share

import (
	"context"
	"encoding/base64"
	"errors"
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

	editor := Access{TenantID: tenant, UserID: 12, ManageTeam: true, Roles: []RoleRef{{ID: role, Name: "Analyst"}}}
	team.Name = "Team default renamed"
	team.DefaultRoleID = nil
	team, err = store.PutView(t.Context(), editor, team)
	require.NoError(t, err)
	require.Equal(t, owner.UserID, team.OwnerUserID, "editing a team view must preserve its original owner")

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
	authorizeErr   error
	exportErr      error
	authorizations int
	calls          int
	stateURL       string
}

func (e *recordingExporter) AuthorizeScheduledExport(_ context.Context, _ ExportSchedule) error {
	e.authorizations++
	return e.authorizeErr
}

func (e *recordingExporter) ExportSavedView(_ context.Context, schedule ExportSchedule) (Workbook, error) {
	e.calls++
	e.stateURL = schedule.StateURL
	if e.exportErr != nil {
		return Workbook{}, e.exportErr
	}
	return Workbook{Filename: "claims.xlsx", Content: []byte("xlsx"), MailSubject: "Weekly claims report", MailBody: "Your scheduled claims report is attached."}, nil
}

type recordingMailer struct {
	mails []Mail
	err   error
}

func (m *recordingMailer) Send(_ context.Context, mail Mail) error {
	m.mails = append(m.mails, mail)
	return m.err
}

type recordingFailureObserver struct{ events []ScheduleFailureEvent }

func (o *recordingFailureObserver) ObserveScheduleFailure(_ context.Context, event ScheduleFailureEvent) {
	o.events = append(o.events, event)
}

func TestScheduleRunnerUsesOperationalDefaults(t *testing.T) {
	t.Parallel()
	runner, err := NewScheduleRunner(NewMemoryStore(), &recordingExporter{}, &recordingMailer{}, ScheduleRunnerConfig{})
	require.NoError(t, err)
	require.Equal(t, 25, runner.batch)
	require.Equal(t, DefaultConsecutiveFailureLimit, runner.failureLimit)
}

type cancelingExporter struct{ cancel context.CancelFunc }

func (e cancelingExporter) AuthorizeScheduledExport(context.Context, ExportSchedule) error {
	return nil
}

func (e cancelingExporter) ExportSavedView(context.Context, ExportSchedule) (Workbook, error) {
	e.cancel()
	return Workbook{Filename: "claims.xlsx", Content: []byte("xlsx"), MailSubject: "Weekly claims report", MailBody: "Your scheduled claims report is attached."}, nil
}

type contextCheckingFinishStore struct {
	Store
	finishContextErr error
}

func (s *contextCheckingFinishStore) FinishSchedule(ctx context.Context, completion ScheduleCompletion) (ExportSchedule, error) {
	s.finishContextErr = ctx.Err()
	if s.finishContextErr != nil {
		return ExportSchedule{}, s.finishContextErr
	}
	return s.Store.FinishSchedule(ctx, completion)
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
	runner, err := NewScheduleRunner(store, exporter, mailer, ScheduleRunnerConfig{BatchSize: 10})
	require.NoError(t, err)
	require.NoError(t, runner.RunDue(t.Context(), dueAt))
	require.Equal(t, 1, exporter.calls)
	require.Equal(t, 1, exporter.authorizations)
	require.Equal(t, "/claims?year=2026&_hidden=paid", exporter.stateURL)
	require.Len(t, mailer.mails, 1)
	require.Equal(t, "claims.xlsx", mailer.mails[0].Attachment.Filename)
	require.Equal(t, "Weekly claims report", mailer.mails[0].Subject)
	require.Equal(t, "Your scheduled claims report is attached.", mailer.mails[0].Body)

	schedules, err := store.ListSchedules(t.Context(), access, "claims")
	require.NoError(t, err)
	require.True(t, schedules[0].LastRunAt.Equal(dueAt))
	require.True(t, schedules[0].NextRunAt.After(dueAt))
}

func TestScheduleRunnerDoesNotExportOrMailAfterAuthorizationRevocation(t *testing.T) {
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
	exporter := &recordingExporter{authorizeErr: errors.New("owner is inactive or no longer has dashboard access")}
	mailer := &recordingMailer{}
	runner, err := NewScheduleRunner(store, exporter, mailer, ScheduleRunnerConfig{BatchSize: 1})
	require.NoError(t, err)
	err = runner.RunDue(t.Context(), schedule.NextRunAt)
	require.ErrorContains(t, err, "scheduled export authorization failed")
	require.Equal(t, 1, exporter.authorizations)
	require.Zero(t, exporter.calls)
	require.Empty(t, mailer.mails)

	schedules, err := store.ListSchedules(t.Context(), access, "claims")
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	require.Contains(t, schedules[0].LastError, "owner is inactive or no longer has dashboard access")
}

func TestScheduleRunnerObservesEachDeliveryFailureAfterCompletion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		stage    ScheduleFailureStage
		exporter *recordingExporter
		mailer   *recordingMailer
	}{
		{name: "authorization", stage: ScheduleFailureStageAuthorization, exporter: &recordingExporter{authorizeErr: errors.New("revoked")}, mailer: &recordingMailer{}},
		{name: "export", stage: ScheduleFailureStageExport, exporter: &recordingExporter{exportErr: errors.New("render failed")}, mailer: &recordingMailer{}},
		{name: "mail", stage: ScheduleFailureStageMail, exporter: &recordingExporter{}, mailer: &recordingMailer{err: errors.New("SMTP failed")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
			observer := &recordingFailureObserver{}
			runner, err := NewScheduleRunner(store, test.exporter, test.mailer, ScheduleRunnerConfig{
				BatchSize: 1, ConsecutiveFailureLimit: 1, FailureObserver: observer,
			})
			require.NoError(t, err)

			require.Error(t, runner.RunDue(t.Context(), schedule.NextRunAt))
			require.Len(t, observer.events, 1)
			require.Equal(t, test.stage, observer.events[0].Stage)
			require.Equal(t, schedule.ID, observer.events[0].Schedule.ID)
			require.Equal(t, 1, observer.events[0].Schedule.ConsecutiveFailures)
			require.False(t, observer.events[0].Schedule.Enabled)
			require.True(t, observer.events[0].Schedule.NextRunAt.IsZero())
			require.Error(t, observer.events[0].Err)
		})
	}
}

func TestMemorySchedulePreservesAndResetsOperationalState(t *testing.T) {
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
		LastError: "client supplied", ConsecutiveFailures: 99, LastRunAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	require.Empty(t, schedule.LastError)
	require.Zero(t, schedule.ConsecutiveFailures)
	require.True(t, schedule.LastRunAt.IsZero())

	due, err := store.DueSchedules(t.Context(), schedule.NextRunAt, 1)
	require.NoError(t, err)
	failed, err := store.FinishSchedule(t.Context(), ScheduleCompletion{
		TenantID: access.TenantID, ScheduleID: schedule.ID, ClaimedUntil: due[0].NextRunAt,
		RanAt: schedule.NextRunAt, NextRunAt: schedule.NextRunAt.Add(7 * 24 * time.Hour),
		RunError: errors.New("delivery failed"), ConsecutiveFailureLimit: 2,
	})
	require.NoError(t, err)
	require.Equal(t, 1, failed.ConsecutiveFailures)

	failed.Name = "Renamed weekly claims"
	edited, err := store.PutSchedule(t.Context(), access, failed)
	require.NoError(t, err)
	require.Equal(t, failed.LastRunAt, edited.LastRunAt)
	require.Equal(t, failed.LastError, edited.LastError)
	require.Equal(t, failed.ConsecutiveFailures, edited.ConsecutiveFailures)
	require.Equal(t, failed.NextRunAt, edited.NextRunAt)

	due, err = store.DueSchedules(t.Context(), edited.NextRunAt, 1)
	require.NoError(t, err)
	disabled, err := store.FinishSchedule(t.Context(), ScheduleCompletion{
		TenantID: access.TenantID, ScheduleID: edited.ID, ClaimedUntil: due[0].NextRunAt,
		RanAt: edited.NextRunAt, NextRunAt: edited.NextRunAt.Add(7 * 24 * time.Hour),
		RunError: errors.New("delivery still failed"), ConsecutiveFailureLimit: 2,
	})
	require.NoError(t, err)
	require.False(t, disabled.Enabled)
	require.Equal(t, 2, disabled.ConsecutiveFailures)
	require.NotEmpty(t, disabled.LastError)

	disabled.Enabled = true
	reenabled, err := store.PutSchedule(t.Context(), access, disabled)
	require.NoError(t, err)
	require.True(t, reenabled.Enabled)
	require.Zero(t, reenabled.ConsecutiveFailures)
	require.Empty(t, reenabled.LastError)
	require.False(t, reenabled.NextRunAt.IsZero())
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
	runner, err := NewScheduleRunner(store, cancelingExporter{cancel: cancel}, &recordingMailer{}, ScheduleRunnerConfig{BatchSize: 1})
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
	completion := ScheduleCompletion{
		TenantID: access.TenantID, ScheduleID: due[0].ID, ClaimedUntil: due[0].NextRunAt,
		RanAt: schedule.NextRunAt, NextRunAt: next, ConsecutiveFailureLimit: 3,
	}
	wrongTenant := completion
	wrongTenant.TenantID = uuid.New()
	_, err = store.FinishSchedule(t.Context(), wrongTenant)
	require.ErrorIs(t, err, ErrClaimLost)
	wrongClaim := completion
	wrongClaim.ClaimedUntil = wrongClaim.ClaimedUntil.Add(time.Second)
	_, err = store.FinishSchedule(t.Context(), wrongClaim)
	require.ErrorIs(t, err, ErrClaimLost)
	_, err = store.FinishSchedule(t.Context(), completion)
	require.NoError(t, err)
	_, err = store.FinishSchedule(t.Context(), completion)
	require.ErrorIs(t, err, ErrClaimLost)
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
