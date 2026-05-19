package spotlight

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEventDeduper_NewKeyIsNotSeen(t *testing.T) {
	t.Parallel()
	d := NewEventDeduper(DefaultEventDedupConfig())
	require.False(t, d.Seen("crm.client", "pk-1", "ev-1"))
	require.True(t, d.Seen("crm.client", "pk-1", "ev-1"), "second arrival within TTL must be seen")
}

func TestEventDeduper_DistinctKeysAreIndependent(t *testing.T) {
	t.Parallel()
	d := NewEventDeduper(DefaultEventDedupConfig())
	require.False(t, d.Seen("crm.client", "pk-1", "ev-1"))
	require.False(t, d.Seen("crm.client", "pk-1", "ev-2"))
	require.False(t, d.Seen("crm.client", "pk-2", "ev-1"))
	require.False(t, d.Seen("insurance.contract", "pk-1", "ev-1"))
}

func TestEventDeduper_TTLExpiresEntries(t *testing.T) {
	t.Parallel()
	d := NewEventDeduper(EventDedupConfig{Capacity: 4, TTL: 5 * time.Millisecond})
	require.False(t, d.Seen("crm.client", "pk", "ev"))
	require.True(t, d.Seen("crm.client", "pk", "ev"))
	time.Sleep(20 * time.Millisecond)
	require.False(t, d.Seen("crm.client", "pk", "ev"), "TTL must release the key")
}

func TestEventDeduper_NilSafe(t *testing.T) {
	t.Parallel()
	var d *EventDeduper
	require.False(t, d.Seen("a", "b", "c"))
	entries, cap := d.Stats()
	require.Zero(t, entries)
	require.Zero(t, cap)
}
