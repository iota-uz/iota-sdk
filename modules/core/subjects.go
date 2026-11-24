package core

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/automation/subjects"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
)

// contributeSubjectNames names a user as the subject of an automation (a human task's record): by
// their full name, with a link to the user's card for a reader with User.Read.
func contributeSubjectNames(b *composition.Builder) {
	subjects.Contribute(b, data.NativeUser, func(c *composition.Container) (subjects.Namer, error) {
		db, err := composition.Resolve[*pgxpool.Pool](c)
		if err != nil {
			return nil, err
		}
		return subjects.TableNamer{
			DB: db, Table: "users", KeyColumn: "id", TenantColumn: "tenant_id",
			Label:          `concat_ws(' ', r.first_name, r.last_name)`,
			ReadPermission: permissions.UserRead.Name(),
			Path:           func(key string) string { return "/users/" + key },
		}, nil
	})
}
