package core

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/automation/subjects"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
)

// contributeSubjectNames names users, roles and groups as the subject of an automation (a human
// task's record): by their name, with a link to their card for a reader with the read permission.
func contributeSubjectNames(b *composition.Builder) {
	for _, n := range []struct {
		entity string
		namer  subjects.TableNamer
	}{
		{"core.role", subjects.TableNamer{Table: "roles", KeyColumn: "id", TenantColumn: "tenant_id", Label: `r.name`,
			ReadPermission: permissions.RoleRead.Name(), Path: func(key string) string { return "/roles/" + key }}},
		{"core.group", subjects.TableNamer{Table: "user_groups", KeyColumn: "id", TenantColumn: "tenant_id", Label: `r.name`,
			ReadPermission: permissions.GroupRead.Name(), Path: func(key string) string { return "/groups/" + key }}},
	} {
		subjects.Contribute(b, n.entity, func(c *composition.Container) (subjects.Namer, error) {
			db, err := composition.Resolve[*pgxpool.Pool](c)
			if err != nil {
				return nil, err
			}
			n.namer.DB = db
			return n.namer, nil
		})
	}
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
