package persistence_test

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/stretchr/testify/require"
)

// Real SQL over real membership rows. Falsely green if every seeded user were
// a group member or active: outsiders, a blocked member, a pending member and
// a member of another tenant all share the search term and must stay out.
func TestNotificationRecipientDirectory_ScopesAndPages(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	tx, err := composables.UseTx(f.Ctx)
	require.NoError(t, err)
	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)
	staff := uuid.New()
	_, err = tx.Exec(f.Ctx, `INSERT INTO user_groups(id,type,name,tenant_id) VALUES($1,'user',$2,$3)`, staff, "Staff "+staff.String(), tenant)
	require.NoError(t, err)
	seed := func(name, email string, tenantID uuid.UUID, status string, blocked, member bool) uint {
		var id uint
		require.NoError(t, tx.QueryRow(f.Ctx, `INSERT INTO users(first_name,last_name,email,ui_language,tenant_id,type,status,is_blocked) VALUES($1,'Zeta',$2,'en',$3,'user',$4,false) RETURNING id`, name, email, tenantID, status).Scan(&id))
		if blocked {
			_, err := tx.Exec(f.Ctx, `UPDATE users SET is_blocked=true, block_reason='test', blocked_at=now(), blocked_by=id, blocked_by_tenant_id=tenant_id WHERE id=$1`, id)
			require.NoError(t, err)
		}
		if member {
			_, err := tx.Exec(f.Ctx, `INSERT INTO group_users(group_id,user_id) VALUES($1,$2)`, staff, id)
			require.NoError(t, err)
		}
		return id
	}
	mail := func(prefix string) string { return fmt.Sprintf("%s-%s@example.test", prefix, uuid.NewString()) }
	first := seed("Alpha", mail("alpha"), tenant, "active", false, true)
	second := seed("Bravo", mail("bravo"), tenant, "active", false, true)
	percent := seed("100%", mail("percent"), tenant, "active", false, true)
	outsider := seed("Outsider", mail("outsider"), tenant, "active", false, false)
	blocked := seed("Blocked", mail("blocked"), tenant, "active", true, true)
	pending := seed("Pending", mail("pending"), tenant, "pending_onboarding", false, true)
	other := uuid.New()
	_, err = tx.Exec(f.Ctx, `INSERT INTO tenants(id,name,domain) VALUES($1,$2,$3)`, other, "Other "+other.String(), other.String()+".test")
	require.NoError(t, err)
	foreign := seed("Foreign", mail("foreign"), other, "active", false, true)

	scoped := persistence.NewNotificationRecipientDirectory(staff)
	ids := func(recipients []notifications.Recipient) []uint {
		result := make([]uint, 0, len(recipients))
		for _, recipient := range recipients {
			result = append(result, recipient.ID)
		}
		return result
	}

	all, err := scoped.Search(f.Ctx, notifications.RecipientQuery{Search: "Zeta", Limit: 50})
	require.NoError(t, err)
	require.Equal(t, []uint{percent, first, second}, ids(all))
	require.Equal(t, "Alpha Zeta", all[1].Name)

	page, err := scoped.Search(f.Ctx, notifications.RecipientQuery{Search: "Zeta", Limit: 1, Offset: 1})
	require.NoError(t, err)
	require.Equal(t, []uint{first}, ids(page))

	both, err := scoped.Search(f.Ctx, notifications.RecipientQuery{Search: "zeta bravo", Limit: 50})
	require.NoError(t, err)
	require.Equal(t, []uint{second}, ids(both))

	literal, err := scoped.Search(f.Ctx, notifications.RecipientQuery{Search: "%", Limit: 50})
	require.NoError(t, err)
	require.Equal(t, []uint{percent}, ids(literal))

	named, err := scoped.GetByIDs(f.Ctx, []uint{first, outsider, blocked, pending, foreign})
	require.NoError(t, err)
	require.Equal(t, []uint{first}, ids(named))

	unscoped := persistence.NewNotificationRecipientDirectory()
	everyone, err := unscoped.Search(f.Ctx, notifications.RecipientQuery{Search: "Zeta", Limit: 50})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint{first, second, percent, outsider}, ids(everyone))
}
