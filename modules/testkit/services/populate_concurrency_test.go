package services_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/testkit/domain/schemas"
	"github.com/iota-uz/iota-sdk/modules/testkit/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestPopulateServiceConcurrentReferences(t *testing.T) {
	// Falsely green if each goroutine gets its own service instead of sharing the public service instance.
	f := setupTest(t)
	service := services.NewPopulateService(f.Pool)
	const count = 4
	results := make([]map[string]interface{}, count)
	errs := make([]error, count)
	tenants := make([]uuid.UUID, count)
	emails := make([]string, count)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range count {
		tenants[i] = uuid.New()
		emails[i] = fmt.Sprintf("concurrent-%d-%s@example.com", i, tenants[i])
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			ctx := context.WithValue(composables.WithTenantID(f.Ctx, tenants[i]), constants.LoggerKey, logrus.NewEntry(logrus.New()))
			results[i], errs[i] = service.Execute(ctx, &schemas.PopulateRequest{Version: "1.0", Tenant: &schemas.TenantSpec{ID: tenants[i].String(), Name: "Concurrent " + tenants[i].String(), Domain: tenants[i].String() + ".test"}, Data: &schemas.DataSpec{Users: []schemas.UserSpec{{Email: emails[i], Password: "TestPass123!", FirstName: "Concurrent", LastName: "User", Ref: "same-reference"}}}, Options: &schemas.OptionsSpec{ReturnIds: true}})
		}(i)
	}
	close(start)
	wg.Wait()
	for i := range count {
		require.NoError(t, errs[i])
		users, ok := results[i]["users"].([]interface{})
		require.True(t, ok)
		require.Len(t, users, 1)
		record, ok := users[0].(map[string]interface{})
		require.True(t, ok)
		require.Equal(t, emails[i], record["email"])
		stored, err := persistence.NewUserRepository(persistence.NewUploadRepository()).GetByEmail(composables.WithTenantID(f.Ctx, tenants[i]), emails[i])
		require.NoError(t, err)
		require.Equal(t, tenants[i], stored.TenantID())
	}
}
