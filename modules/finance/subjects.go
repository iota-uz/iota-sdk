package finance

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/pkg/automation/subjects"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

// contributeSubjectNames names the module's records when an automation is about one (a human task's
// record): by the record's name, with a link to its card for a reader allowed to open it.
func contributeSubjectNames(b *composition.Builder) {
	for _, n := range []struct {
		entity string
		namer  subjects.TableNamer
	}{
		// A payment and an expense have no name of their own: the entity names them, the link opens them.
		{"finance.payment", subjects.TableNamer{Table: "payments", KeyColumn: "id", TenantColumn: "tenant_id",
			Label: `''`, ReadPermission: permissions.PaymentRead.Name(),
			Path: func(key string) string { return "/finance/payments/" + key }}},
		{"finance.expense", subjects.TableNamer{Table: "expenses", KeyColumn: "id", TenantColumn: "tenant_id",
			Label: `''`, ReadPermission: permissions.ExpenseRead.Name(),
			Path: func(key string) string { return "/finance/expenses/" + key }}},
	} {
		n := n
		subjects.Contribute(b, n.entity, func(c *composition.Container) (subjects.Namer, error) {
			db, err := composition.Resolve[*pgxpool.Pool](c)
			if err != nil {
				return nil, err
			}
			n.namer.DB = db
			return n.namer, nil
		})
	}
}
