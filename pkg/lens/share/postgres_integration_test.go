package share_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/lens/share"
	"github.com/stretchr/testify/require"
)

func TestPostgresStoreOwnershipConcurrentLeaseAndClaimLoss(t *testing.T) {
	t.Chdir("../../..")
	suite := itf.NewSuiteBuilder(t).Build()
	environment := suite.Env()
	store, err := share.NewPostgresStore(environment.Pool)
	require.NoError(t, err)
	var ownerID uint
	err = environment.Pool.QueryRow(environment.Ctx, `
		INSERT INTO users (first_name, last_name, email, ui_language, tenant_id, type)
		VALUES ('Lens', 'Owner', $1, 'en', $2, 'user') RETURNING id`,
		"lens-"+environment.Tenant.ID.String()+"@example.com", environment.Tenant.ID,
	).Scan(&ownerID)
	require.NoError(t, err)
	access := share.Access{
		TenantID:     environment.Tenant.ID,
		UserID:       ownerID,
		ManageTeam:   true,
		ScheduleMail: true,
	}
	view, err := store.PutView(environment.Ctx, access, share.SavedView{
		DashboardID: "audit", Name: "Team view", Scope: share.ViewScopeTeam, StateURL: "/audit?period=month",
	})
	require.NoError(t, err)

	manager := access
	manager.UserID++
	personalView, err := store.PutView(environment.Ctx, access, share.SavedView{
		DashboardID: "audit", Name: "Personal view", Scope: share.ViewScopePersonal, StateURL: "/audit?period=week",
	})
	require.NoError(t, err)
	_, err = store.PutView(environment.Ctx, manager, share.SavedView{
		ID: personalView.ID, DashboardID: personalView.DashboardID, Name: "Unauthorized rename",
		Scope: share.ViewScopePersonal, StateURL: personalView.StateURL,
	})
	require.ErrorIs(t, err, share.ErrNotFound)

	updated, err := store.PutView(environment.Ctx, manager, share.SavedView{
		ID: view.ID, DashboardID: view.DashboardID, Name: "Renamed by manager",
		Scope: share.ViewScopeTeam, StateURL: view.StateURL,
	})
	require.NoError(t, err)
	require.Equal(t, access.UserID, updated.OwnerUserID)

	schedule, err := store.PutSchedule(environment.Ctx, access, share.ExportSchedule{
		DashboardID: view.DashboardID, ViewID: view.ID, Name: "Hourly audit",
		Cron: "0 * * * *", Timezone: "UTC", Recipients: []string{"audit@example.com"}, Enabled: true,
	})
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Microsecond)
	_, err = environment.Pool.Exec(environment.Ctx,
		`UPDATE lens_export_schedules SET next_run_at = $1 WHERE id = $2`, now.Add(-time.Minute), schedule.ID)
	require.NoError(t, err)

	results := make(chan []share.ExportSchedule, 2)
	errorsOut := make(chan error, 2)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			due, dueErr := store.DueSchedules(context.Background(), now, 1)
			results <- due
			errorsOut <- dueErr
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	close(errorsOut)
	for dueErr := range errorsOut {
		require.NoError(t, dueErr)
	}
	claimed := make([]share.ExportSchedule, 0, 1)
	for due := range results {
		claimed = append(claimed, due...)
	}
	require.Len(t, claimed, 1, "SKIP LOCKED must lease a due schedule to only one worker")

	claim := claimed[0]
	completion := share.ScheduleCompletion{
		TenantID: access.TenantID, ScheduleID: claim.ID, ClaimedUntil: claim.NextRunAt,
		RanAt: now, NextRunAt: now.Add(time.Hour), ConsecutiveFailureLimit: 3,
	}
	wrongClaim := completion
	wrongClaim.ClaimedUntil = wrongClaim.ClaimedUntil.Add(time.Second)
	_, err = store.FinishSchedule(environment.Ctx, wrongClaim)
	require.ErrorIs(t, err, share.ErrClaimLost)
	completion.RunError = errors.New("delivery failed")
	_, err = store.FinishSchedule(environment.Ctx, completion)
	require.NoError(t, err)

	schedules, err := store.ListSchedules(environment.Ctx, access, view.DashboardID)
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	require.Equal(t, "delivery failed", schedules[0].LastError)
	require.Equal(t, 1, schedules[0].ConsecutiveFailures)
	require.True(t, schedules[0].Enabled)
	schedules[0].Name = "Renamed hourly audit"
	edited, err := store.PutSchedule(environment.Ctx, access, schedules[0])
	require.NoError(t, err)
	require.Equal(t, schedules[0].LastRunAt, edited.LastRunAt)
	require.Equal(t, schedules[0].LastError, edited.LastError)
	require.Equal(t, schedules[0].ConsecutiveFailures, edited.ConsecutiveFailures)
	require.Equal(t, schedules[0].NextRunAt, edited.NextRunAt)

	due, err := store.DueSchedules(environment.Ctx, edited.NextRunAt, 1)
	require.NoError(t, err)
	require.Len(t, due, 1)
	succeeded, err := store.FinishSchedule(environment.Ctx, share.ScheduleCompletion{
		TenantID: access.TenantID, ScheduleID: due[0].ID, ClaimedUntil: due[0].NextRunAt,
		RanAt: edited.NextRunAt, NextRunAt: edited.NextRunAt.Add(time.Hour),
		ConsecutiveFailureLimit: 3,
	})
	require.NoError(t, err)
	require.Zero(t, succeeded.ConsecutiveFailures)
	require.Empty(t, succeeded.LastError)

	current := succeeded
	for attempt := 1; attempt <= 3; attempt++ {
		due, err = store.DueSchedules(environment.Ctx, current.NextRunAt, 1)
		require.NoError(t, err)
		require.Len(t, due, 1)
		current, err = store.FinishSchedule(environment.Ctx, share.ScheduleCompletion{
			TenantID: access.TenantID, ScheduleID: due[0].ID, ClaimedUntil: due[0].NextRunAt,
			RanAt: current.NextRunAt, NextRunAt: current.NextRunAt.Add(time.Hour),
			RunError: errors.New("persistent delivery failure"), ConsecutiveFailureLimit: 3,
		})
		require.NoError(t, err)
		require.Equal(t, attempt, current.ConsecutiveFailures)
	}
	require.False(t, current.Enabled)
	require.True(t, current.NextRunAt.IsZero())
	require.Equal(t, "persistent delivery failure", current.LastError)

	current.Enabled = true
	reenabled, err := store.PutSchedule(environment.Ctx, access, current)
	require.NoError(t, err)
	require.True(t, reenabled.Enabled)
	require.Zero(t, reenabled.ConsecutiveFailures)
	require.Empty(t, reenabled.LastError)
	require.False(t, reenabled.NextRunAt.IsZero())
}
