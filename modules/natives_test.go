package modules_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

// requiredEventLocales are the locales every SDK event is labeled and
// described in. Applications check their own locales in their own tests.
var requiredEventLocales = []string{"en", "ru", "uz", "uz-Cyrl"}

// eventPublisherTests names, for every SDK event, a test of a publisher that
// shows the event committing with its change and rolling back with it
// (relative to this directory, "file:TestName").
var eventPublisherTests = map[string]string{
	"core.user.created.v1":  "core/services/user_events_test.go:TestUserService_Create_PublishesUserCreatedEventInTheCreatingTransaction",
	"crm.client.created.v1": "crm/services/client_events_test.go:TestClientEvents_CommitAndRollBackWithTheClient",
	"crm.client.updated.v1": "crm/services/client_events_test.go:TestClientEvents_CommitAndRollBackWithTheClient",

	"finance.payment.created.v1":  "finance/services/events_test.go:TestPaymentEvents_CommitAndRollBackWithThePayment",
	"finance.payment.updated.v1":  "finance/services/events_test.go:TestPaymentEvents_CommitAndRollBackWithThePayment",
	"finance.expense.created.v1":  "finance/services/events_test.go:TestExpenseEvents_CommitAndRollBackWithTheExpense",
	"finance.debt.created.v1":     "finance/services/events_test.go:TestDebtEvents_CommitAndRollBackWithTheDebt",
	"finance.debt.settled.v1":     "finance/services/events_test.go:TestDebtEvents_CommitAndRollBackWithTheDebt",
	"finance.debt.written_off.v1": "finance/services/events_test.go:TestDebtEvents_CommitAndRollBackWithTheDebt",
	"finance.debt.cancelled.v1":   "finance/services/events_test.go:TestDebtEvents_CommitAndRollBackWithTheDebt",

	"billing.transaction.status_changed.v1": "billing/services/events_test.go:TestBillingEvents_StatusChangeCommitsAndRollsBackWithTheTransaction",

	"warehouse.order.created.v1":   "warehouse/services/orderservice/events_test.go:TestOrderEvents_CommitAndRollBackWithTheOrder",
	"warehouse.order.completed.v1": "warehouse/services/orderservice/events_test.go:TestOrderEvents_CommitAndRollBackWithTheOrder",

	"projects.acceptance.signed.v1": "projects/services/acceptance_events_test.go:TestAcceptanceEvents_SignedCommitsAndRollsBackWithTheDocument",
}

var publisherTestRef = regexp.MustCompile(`^([^:]+_test\.go):(Test\w+)$`)

// Every event of modules.Natives() follows the shared key grammar, is labeled
// and described in every required locale and has a publisher with a
// commit/rollback test.
func TestNativeEventsCoverage(t *testing.T) {
	set, err := native.Compose(modules.Natives()...)
	require.NoError(t, err)
	evs := set.Events()
	require.NotEmpty(t, evs)
	for _, ev := range evs {
		t.Run(ev.ID(), func(t *testing.T) {
			require.True(t, native.ValidEventKey(ev.ID()), "the key follows %s", native.EventKeyPattern)
			meta, ok := ev.(native.EventMetadata)
			require.True(t, ok, "a native event carries its metadata")
			for _, locale := range requiredEventLocales {
				require.NotEmpty(t, meta.Label()[locale], "Label[%s]", locale)
				require.NotEmpty(t, meta.Descriptions()[locale], "Descriptions[%s]", locale)
			}
			ref, ok := eventPublisherTests[ev.ID()]
			require.True(t, ok, "name the commit/rollback test of a publisher in eventPublisherTests")
			m := publisherTestRef.FindStringSubmatch(ref)
			require.NotNil(t, m, "eventPublisherTests entry %q is file_test.go:TestName", ref)
			src, err := os.ReadFile(filepath.FromSlash(m[1]))
			require.NoError(t, err)
			require.Regexp(t, `(?m)^func `+m[2]+`\(t \*testing\.T\)`, string(src), "the publisher test exists")
		})
	}
	for id := range eventPublisherTests {
		_, ok := set.Event(id)
		require.True(t, ok, "eventPublisherTests names an event that is no longer declared: %s", id)
	}
}
