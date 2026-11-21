package host

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// UserSubjects loads the current rights of a user (direct, role and group
// grants). It serves triggers (events, schedules, workflow runs) as a
// SubjectLoader and contact resolvers as a notify.SubjectResolver. It needs no
// extension runtime, so the core module's Go-only outbox dispatcher uses it
// too.
type UserSubjects struct {
	Users    user.Repository
	Universe []permission.Permission
	// Pool is put into the context of repository reads: triggers run in
	// background goroutines whose context carries no database.
	Pool *pgxpool.Pool
}

var _ SubjectLoader = (*UserSubjects)(nil)

func (s *UserSubjects) load(ctx context.Context, tenantID uuid.UUID, userID int64, kind spi.SubjectKind) (spi.Subject, error) {
	if userID <= 0 {
		return spi.Subject{}, &SubjectBlockedError{TenantID: tenantID, UserID: userID, Reason: "no user"}
	}
	ctx = composables.WithTenantID(ctx, tenantID)
	if s.Pool != nil && ctx.Value(constants.TxKey) == nil {
		ctx = composables.WithPool(ctx, s.Pool)
	}
	u, err := s.Users.GetByID(ctx, uint(userID))
	if err != nil {
		if serrors.CodeOf(err) == serrors.NotFound {
			return spi.Subject{}, &SubjectBlockedError{TenantID: tenantID, UserID: userID, Reason: "user not found"}
		}
		return spi.Subject{}, err
	}
	if u.IsBlocked() {
		return spi.Subject{}, &SubjectBlockedError{TenantID: tenantID, UserID: userID, Reason: "user is blocked"}
	}
	return spi.Subject{
		TenantID: tenantID, UserID: userID, Kind: kind,
		Permissions: PermissionNames(user.EffectivePermissions(u)), Locale: string(u.UILanguage()),
	}, nil
}

// LoadSubject implements SubjectLoader.
func (s *UserSubjects) LoadSubject(ctx context.Context, tenantID uuid.UUID, userID int64, kind spi.SubjectKind) (spi.Subject, error) {
	return s.load(ctx, tenantID, userID, kind)
}

// ResolveSubject implements notify.SubjectResolver.
func (s *UserSubjects) ResolveSubject(ctx context.Context, tenantID uuid.UUID, userID int64) (spi.Subject, error) {
	return s.load(ctx, tenantID, userID, spi.SubjectOwner)
}

// PermissionNames lists the RBAC names of ps.
func PermissionNames(ps []permission.Permission) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Name())
	}
	return out
}
