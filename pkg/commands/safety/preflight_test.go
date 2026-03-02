package safety

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunPreflight_HostClassificationAndBackupMarker(t *testing.T) {
	conf := configuration.Use()
	origHost := conf.Database.Host
	origName := conf.Database.Name
	defer func() {
		conf.Database.Host = origHost
		conf.Database.Name = origName
	}()

	conf.Database.Host = "db.example.com"
	conf.Database.Name = "iota_prod_backup"

	res, err := RunPreflight(context.Background(), nil, OperationSeedMain)
	require.NoError(t, err)
	assert.False(t, res.DBState.IsLocalHost)
	assert.True(t, res.DBState.LooksLikeBackup)
	assert.True(t, res.HasHighRisk())
	assert.Contains(t, res.Actions, "seed users")
}

func TestRunPreflight_LocalHostNoImplicitRisk(t *testing.T) {
	conf := configuration.Use()
	origHost := conf.Database.Host
	origName := conf.Database.Name
	defer func() {
		conf.Database.Host = origHost
		conf.Database.Name = origName
	}()

	conf.Database.Host = "localhost"
	conf.Database.Name = "iota_erp"

	res, err := RunPreflight(context.Background(), nil, OperationSeedSuperadmin)
	require.NoError(t, err)
	assert.True(t, res.DBState.IsLocalHost)
	assert.False(t, res.DBState.LooksLikeBackup)
}
