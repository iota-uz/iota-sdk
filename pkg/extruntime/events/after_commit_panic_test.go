package events

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

// A panicking after-commit callback of a committed delivery is logged and does
// not stop the callbacks after it.
func TestRunAfterCommitIsolatesPanics(t *testing.T) {
	d := &Dispatcher{cfg: Config{Logger: slog.New(slog.DiscardHandler)}}
	var ran []string
	for _, fn := range []func(context.Context){
		func(context.Context) { ran = append(ran, "first") },
		func(context.Context) { panic("a failing callback") },
		func(context.Context) { ran = append(ran, "third") },
	} {
		d.runAfterCommit(context.Background(), fn)
	}
	require.Equal(t, []string{"first", "third"}, ran)
}
