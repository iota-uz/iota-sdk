package tasks

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A page that fills at the end of a batch keeps the position of its own last task: the next page
// starts right after it, also when the next match is in the next batch.
func TestFillDecidedAcrossBatches(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	all := make([]decidedPos, 5)
	for i := range all {
		all[i] = decidedPos{id: uuid.New(), at: base.Add(-time.Duration(i) * time.Minute)}
	}
	read := func(after *decidedPos, limit int) ([]decidedPos, error) {
		start := 0
		if after != nil {
			for i, p := range all {
				if p.id == after.id {
					start = i + 1
				}
			}
		}
		return all[start:min(start+limit, len(all))], nil
	}
	shown := func(list []decidedPos) (map[uuid.UUID]TaskView, error) {
		out := map[uuid.UUID]TaskView{}
		for _, p := range list {
			out[p.id] = TaskView{ID: p.id}
		}
		return out, nil
	}
	var got []uuid.UUID
	var after *decidedPos
	for range 5 {
		page, err := fillDecided(2, 2, after, read, shown)
		require.NoError(t, err)
		for _, v := range page.Items {
			got = append(got, v.ID)
		}
		if page.NextCursor == "" {
			break
		}
		at, id, err := parseDecidedCursor(page.NextCursor)
		require.NoError(t, err)
		require.False(t, at.IsZero(), "the cursor carries the time of the page's last task")
		after = &decidedPos{id: id, at: at}
	}
	want := make([]uuid.UUID, len(all))
	for i, p := range all {
		want[i] = p.id
	}
	assert.Equal(t, want, got, "every decision once, in order")
}
