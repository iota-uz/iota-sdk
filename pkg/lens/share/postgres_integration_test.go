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
	err = store.FinishSchedule(environment.Ctx, access.TenantID, claim.ID,
		claim.NextRunAt.Add(time.Second), now, now.Add(time.Hour), nil)
	require.ErrorIs(t, err, share.ErrClaimLost)
	err = store.FinishSchedule(environment.Ctx, access.TenantID, claim.ID,
		claim.NextRunAt, now, now.Add(time.Hour), errors.New("delivery failed"))
	require.NoError(t, err)

	schedules, err := store.ListSchedules(environment.Ctx, access, view.DashboardID)
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	require.Equal(t, "delivery failed", schedules[0].LastError)
}
