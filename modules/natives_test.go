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
	"core.user.created.v1":            "core/services/user_events_test.go:TestUserService_Create_PublishesUserCreatedEventInTheCreatingTransaction",
	"core.user.updated.v1":            "core/services/native_events_test.go:TestUserEvents_CommitAndRollBackWithTheChange",
	"core.user.blocked.v1":            "core/services/native_events_test.go:TestUserEvents_CommitAndRollBackWithTheChange",
	"core.user.unblocked.v1":          "core/services/native_events_test.go:TestUserEvents_CommitAndRollBackWithTheChange",
	"core.user.deleted.v1":            "core/services/native_events_test.go:TestUserEvents_CommitAndRollBackWithTheChange",
	"core.user.role_assigned.v1":      "core/services/native_events_test.go:TestUserEvents_CommitAndRollBackWithTheChange",
	"core.role.created.v1":            "core/services/native_events_test.go:TestRoleEvents_CommitAndRollBackWithTheChange",
	"core.role.permission_granted.v1": "core/services/native_events_test.go:TestRoleEvents_CommitAndRollBackWithTheChange",
	"core.group.created.v1":           "core/services/native_events_test.go:TestGroupEvents_CommitAndRollBackWithTheChange",
	"core.group.member_added.v1":      "core/services/native_events_test.go:TestGroupEvents_CommitAndRollBackWithTheChange",
	"core.group.member_removed.v1":    "core/services/native_events_test.go:TestGroupEvents_CommitAndRollBackWithTheChange",
	"crm.client.created.v1":           "crm/services/client_events_test.go:TestClientEvents_CommitAndRollBackWithTheClient",
	"crm.client.updated.v1":           "crm/services/client_events_test.go:TestClientEvents_CommitAndRollBackWithTheClient",
	"crm.client.deleted.v1":           "crm/services/native_events_test.go:TestClientServiceEvents_CommitAndRollBackWithTheClient",
	"crm.chat.message_added.v1":       "crm/services/native_events_test.go:TestChatEvents_MessageAddedCommitsAndRollsBackWithTheMessage",
	"hrm.employee.created.v1":         "hrm/services/native_events_test.go:TestEmployeeEvents_CommitAndRollBackWithTheEmployee",
	"hrm.employee.updated.v1":         "hrm/services/native_events_test.go:TestEmployeeEvents_CommitAndRollBackWithTheEmployee",
}

// contractHashes pins the contracts that existed before the events of #1332:
// their payloads did not change, so their hashes must not.
var contractHashes = map[string]string{
	"core.user.created.v1":  "e3d9470226f2757ef9269af0cfa055a77c89fcc50352549de67ee3d227f7e569",
	"crm.client.created.v1": "63b4839536ba51abf135cd79186d77ddfb8cd63a6c417639441282cb69b550e4",
	"crm.client.updated.v1": "b919391660de57675d516e31d65e527f8ad018c02576bbf2c222ce64a29f8c46",
}

func TestNativeEventContractsAreUnchanged(t *testing.T) {
	set, err := native.Compose(modules.Natives()...)
	require.NoError(t, err)
	for id, want := range contractHashes {
		ev, ok := set.Event(id)
		require.True(t, ok, id)
		require.Equal(t, want, ev.Resource().ContractHash, "%s: a payload change is a new event ID", id)
	}
}

// Every subject of an event is a native entity of the set, with a read
// permission, so the palette and the catalog can resolve it.
func TestNativeEventSubjectsAreEntities(t *testing.T) {
	set, err := native.Compose(modules.Natives()...)
	require.NoError(t, err)
	for _, ev := range set.Events() {
		res := ev.Resource()
		require.NotEmpty(t, res.Subject, ev.ID())
		_, ok := set.EntityRegistry().Get(res.Subject)
		require.True(t, ok, "%s: subject %s is a native entity", ev.ID(), res.Subject)
		meta, ok := ev.(native.EventMetadata)
		require.True(t, ok)
		require.NotEmpty(t, meta.ReadPermission(), ev.ID())
		require.NotEmpty(t, meta.Category(), ev.ID())
	}
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
