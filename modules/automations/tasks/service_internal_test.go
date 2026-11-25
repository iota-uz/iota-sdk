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
	all := make([]DecidedPosition, 5)
	for i := range all {
		all[i] = DecidedPosition{TaskID: uuid.New(), At: base.Add(-time.Duration(i) * time.Minute)}
	}
	read := func(after *DecidedPosition, limit int) ([]DecidedPosition, error) {
		start := 0
		if after != nil {
			for i, p := range all {
				if p.TaskID == after.TaskID {
					start = i + 1
				}
			}
		}
		return all[start:min(start+limit, len(all))], nil
	}
	shown := func(list []DecidedPosition) (map[uuid.UUID]TaskView, error) {
		out := map[uuid.UUID]TaskView{}
		for _, p := range list {
			out[p.TaskID] = TaskView{ID: p.TaskID}
		}
		return out, nil
	}
	var got []uuid.UUID
	var after *DecidedPosition
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
		lastID := page.Items[len(page.Items)-1].ID
		require.Equal(t, lastID, id, "the cursor is after the page's last task")
		for _, p := range all {
			if p.TaskID == lastID {
				require.True(t, p.At.Equal(at), "the cursor carries the exact time of the page's last task")
			}
		}
		after = &DecidedPosition{TaskID: id, At: at}
	}
	want := make([]uuid.UUID, len(all))
	for i, p := range all {
		want[i] = p.TaskID
	}
	assert.Equal(t, want, got, "every decision once, in order")
}
