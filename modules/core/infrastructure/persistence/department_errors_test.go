package persistence

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDepartmentConstraintPreservesIdentityAndPublicField(t *testing.T) {
	driver := &pgconn.PgError{Code: "23505", ConstraintName: departmentsTenantCodeUniqueConstraint, Detail: "private tenant data"}
	err := classifyDepartmentDBError("department.Save", driver)
	require.ErrorIs(t, err, driver)
	require.ErrorIs(t, err, department.ErrDuplicateCode)
	require.Equal(t, serrors.AlreadyExists, serrors.CodeOf(err))
	require.Equal(t, "Code", serrors.FieldsOf(err)[0].Field)
	require.NotContains(t, serrors.Public(err, nil).Message, "private")
	unknown := &pgconn.PgError{Code: "23505", ConstraintName: "unrecognized_constraint"}
	require.Equal(t, serrors.Internal, serrors.CodeOf(classifyDepartmentDBError("department.Save", unknown)))
	require.ErrorIs(t, classifyDepartmentDBError("department.Save", unknown), unknown)
}
