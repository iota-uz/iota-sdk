package crm

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/pkg/automation/subjects"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
)

// contributeSubjectNames names a client as the subject of an automation (a human task's record): by
// its full name, with a link to the client's card for a reader with Client.Read.
func contributeSubjectNames(b *composition.Builder) {
	subjects.Contribute(b, data.NativeClient, func(c *composition.Container) (subjects.Namer, error) {
		db, err := composition.Resolve[*pgxpool.Pool](c)
		if err != nil {
			return nil, err
		}
		return subjects.TableNamer{
			DB: db, Table: "clients", KeyColumn: "id", TenantColumn: "tenant_id",
			Label:          `concat_ws(' ', r.first_name, r.last_name)`,
			ReadPermission: permissions.ClientRead.Name(),
			Path:           func(key string) string { return ClientsPath + "/" + key },
		}, nil
	})
}
