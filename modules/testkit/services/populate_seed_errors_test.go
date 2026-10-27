package services

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"testing"
)

type failedSeedRoles struct {
	role.Repository
	failure error
}

func (r failedSeedRoles) GetPaginated(context.Context, *role.FindParams) ([]role.Role, error) {
	return nil, r.failure
}

type failedSeedPermissions struct {
	permission.Repository
	failure error
	writes  int
}

func (r *failedSeedPermissions) Save(context.Context, permission.Permission) error {
	r.writes++
	return r.failure
}

func TestEnsureAdminRolePropagatesFirstFailure(t *testing.T) {
	for _, lookupFailure := range []bool{false, true} {
		t.Run(map[bool]string{false: "permission write", true: "role lookup"}[lookupFailure], func(t *testing.T) {
			original := errors.New("original repository failure")
			roles := failedSeedRoles{}
			permissions := &failedSeedPermissions{failure: original}
			if lookupFailure {
				roles.failure = original
			}
			ctx := context.WithValue(context.Background(), constants.LoggerKey, logrus.NewEntry(logrus.New()))
			_, err := (&PopulateService{}).ensureAdminRole(ctx, roles, permissions, uuid.New())
			require.ErrorIs(t, err, original)
			if lookupFailure {
				require.Zero(t, permissions.writes)
			} else {
				require.Equal(t, 1, permissions.writes)
			}
		})
	}
}
