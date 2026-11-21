package events_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type recordingTx struct {
	repo.Tx
	mu    sync.Mutex
	execs []string
}

func (tx *recordingTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.execs = append(tx.execs, sql)
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (tx *recordingTx) inserts() int {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	n := 0
	for _, sql := range tx.execs {
		if strings.Contains(sql, "INSERT INTO extruntime.outbox_events") {
			n++
		}
	}
	return n
}

type publishTxPayload struct {
	ThingID string `json:"thingId" wire:"uuid"`
}

var publishTxEvent = native.Event[publishTxPayload]("test.thing.created.v1", native.EventSpec{
	Subject: "test.thing",
	Label:   manifest.Label{"en": "Thing created"},
})

func TestPublishInTx_NeedsATenantAndATransaction(t *testing.T) {
	tx := &recordingTx{}
	ctx := context.WithValue(context.Background(), constants.TxKey, tx)
	ref := &spi.ObjectRef{Entity: "test.thing", Key: "1"}
	payload := publishTxPayload{ThingID: uuid.NewString()}

	require.ErrorContains(t, events.PublishInTx(ctx, uuid.Nil, publishTxEvent, ref, payload), "no tenant")
	require.Zero(t, tx.inserts(), "an event without a tenant is refused before it is written")

	require.NoError(t, events.PublishInTx(ctx, uuid.New(), publishTxEvent, ref, payload))
	require.Equal(t, 1, tx.inserts(), "the event is written in the transaction of ctx")

	require.Error(t, events.PublishInTx(ctx, uuid.New(), publishTxEvent, nil, payload), "the subject is checked against the declaration")
	require.Error(t, events.PublishInTx(context.Background(), uuid.New(), publishTxEvent, ref, payload), "without a transaction or pool there is nowhere to write")
}
