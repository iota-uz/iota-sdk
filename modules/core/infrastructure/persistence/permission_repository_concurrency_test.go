package persistence_test

import (
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

func TestPermissionSaveConcurrentFirstInsertKeepsTransactionsUsable(t *testing.T) {
	f := setupTest(t)
	repository := persistence.NewPermissionRepository()
	p := permission.MustCreate(uuid.New(), "Concurrency.Read", "concurrency", permission.ActionRead, permission.ModifierAll)
	const count = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	failures := make([]error, count)
	for i := range count {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tx, err := f.Pool.Begin(f.Ctx)
			if err != nil {
				failures[i] = err
				return
			}
			defer func() { _ = tx.Rollback(f.Ctx) }()
			<-start
			ctx := composables.WithTx(f.Ctx, tx)
			if err = repository.Save(ctx, p); err != nil {
				failures[i] = err
				return
			}
			var value int
			if err = tx.QueryRow(ctx, "SELECT 1").Scan(&value); err != nil {
				failures[i] = err
				return
			}
			failures[i] = tx.Commit(ctx)
		}(i)
	}
	close(start)
	wg.Wait()
	for _, err := range failures {
		require.NoError(t, err)
	}
	stored, err := repository.GetByID(f.Ctx, p.ID().String())
	require.NoError(t, err)
	require.Equal(t, p.Name(), stored.Name())
}

func TestPermissionSaveRetainsNameUpsertAndRejectsDifferentNameIDCollision(t *testing.T) {
	f := setupTest(t)
	repository := persistence.NewPermissionRepository()
	first := permission.MustCreate(uuid.New(), "Identity.Read", "identity", permission.ActionRead, permission.ModifierAll)
	require.NoError(t, repository.Save(f.Ctx, first))
	replacement := permission.MustCreate(uuid.New(), first.Name(), "identity", permission.ActionRead, permission.ModifierAll)
	require.NoError(t, repository.Save(f.Ctx, replacement))
	stored, err := repository.GetByID(f.Ctx, replacement.ID().String())
	require.NoError(t, err)
	require.Equal(t, first.Name(), stored.Name())
	collision := permission.MustCreate(replacement.ID(), "Collision.Read", "collision", permission.ActionRead, permission.ModifierAll)
	err = repository.Save(f.Ctx, collision)
	var driver *pgconn.PgError
	require.ErrorAs(t, err, &driver)
	require.Equal(t, "23505", driver.Code)
	require.Equal(t, "permissions_pkey", driver.ConstraintName)
}
