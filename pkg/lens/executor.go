package lens

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Results holds the executed query results for every panel in a dashboard.
type Results struct {
	Panels   map[string]*PanelResult
	Duration time.Duration
}

// PanelResult holds the query result (or error) for a single panel.
type PanelResult struct {
	Data     *QueryResult
	Error    error
	Duration time.Duration
}

// Execute runs all panel queries in the dashboard concurrently and returns the
// collected results keyed by panel ID. Panels without a query are skipped.
func Execute(ctx context.Context, ds DataSource, dash Dashboard) *Results {
	start := time.Now()
	res := &Results{
		Panels: make(map[string]*PanelResult),
	}

	// Collect all panels with queries.
	type panelRef struct {
		id    string
		query string
	}
	var refs []panelRef
	for _, row := range dash.Rows {
		for _, p := range row.Panels {
			if p.Query != "" {
				refs = append(refs, panelRef{id: p.ID, query: p.Query})
			}
		}
	}

	if len(refs) == 0 {
		res.Duration = time.Since(start)
		return res
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, ref := range refs {
		wg.Add(1)
		go func(id, query string) {
			defer wg.Done()

			qStart := time.Now()
			data, err := ds.Execute(ctx, query)
			dur := time.Since(qStart)
			if dur == 0 {
				dur = 1
			}
			pr := &PanelResult{
				Data:     data,
				Error:    err,
				Duration: dur,
			}
			if err != nil {
				pr.Error = fmt.Errorf("panel %s: %w", id, err)
			}

			mu.Lock()
			res.Panels[id] = pr
			mu.Unlock()
		}(ref.id, ref.query)
	}

	wg.Wait()
	res.Duration = time.Since(start)
	if res.Duration == 0 {
		res.Duration = 1
	}
	return res
}
