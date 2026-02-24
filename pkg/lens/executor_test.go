package lens

import (
	"context"
	"errors"
	"testing"
)

// ---------------------------------------------------------------------------
// Mock DataSource
// ---------------------------------------------------------------------------

// mockDataSource is a simple in-memory DataSource for testing.
// It returns a pre-configured result for each query string. If the query is
// not found in results it returns errNotFound; if the query is found in errors
// it returns that error instead.
type mockDataSource struct {
	results map[string]*QueryResult
	errors  map[string]error
}

func (m *mockDataSource) Execute(_ context.Context, query string) (*QueryResult, error) {
	if err, ok := m.errors[query]; ok {
		return nil, err
	}
	if r, ok := m.results[query]; ok {
		return r, nil
	}
	return &QueryResult{}, nil
}

func (m *mockDataSource) Close() error { return nil }

// ---------------------------------------------------------------------------
// TestExecute_Concurrent
// ---------------------------------------------------------------------------

func TestExecute_Concurrent(t *testing.T) {
	t.Parallel()

	q1 := "SELECT 1 AS value"
	q2 := "SELECT 2 AS value"
	q3 := "SELECT 3 AS value"

	ds := &mockDataSource{
		results: map[string]*QueryResult{
			q1: {
				Columns: []QueryColumn{{Name: "value", Type: "number"}},
				Rows:    []map[string]any{{"value": float64(1)}},
			},
			q2: {
				Columns: []QueryColumn{{Name: "value", Type: "number"}},
				Rows:    []map[string]any{{"value": float64(2)}},
			},
			q3: {
				Columns: []QueryColumn{{Name: "value", Type: "number"}},
				Rows:    []map[string]any{{"value": float64(3)}},
			},
		},
		errors: map[string]error{},
	}

	dash := NewDashboard("Test",
		NewRow(
			Metric("p1", "Panel 1").Query(q1).Build(),
			Metric("p2", "Panel 2").Query(q2).Build(),
		),
		NewRow(
			Metric("p3", "Panel 3").Query(q3).Build(),
		),
	)

	results := Execute(context.Background(), ds, dash)

	if results == nil {
		t.Fatal("expected non-nil results")
	}
	if len(results.Panels) != 3 {
		t.Fatalf("expected 3 panel results, got %d", len(results.Panels))
	}

	for _, id := range []string{"p1", "p2", "p3"} {
		pr, ok := results.Panels[id]
		if !ok {
			t.Errorf("expected result for panel %q, not found", id)
			continue
		}
		if pr.Error != nil {
			t.Errorf("panel %q: unexpected error: %v", id, pr.Error)
		}
		if pr.Data == nil {
			t.Errorf("panel %q: expected non-nil Data", id)
		}
	}
}

// ---------------------------------------------------------------------------
// TestExecute_PanelError
// ---------------------------------------------------------------------------

func TestExecute_PanelError(t *testing.T) {
	t.Parallel()

	goodQuery := "SELECT 1 AS value"
	badQuery := "SELECT boom"
	sentinelErr := errors.New("db connection failed")

	ds := &mockDataSource{
		results: map[string]*QueryResult{
			goodQuery: {
				Columns: []QueryColumn{{Name: "value", Type: "number"}},
				Rows:    []map[string]any{{"value": float64(42)}},
			},
		},
		errors: map[string]error{
			badQuery: sentinelErr,
		},
	}

	dash := NewDashboard("Test",
		NewRow(
			Metric("ok", "OK Panel").Query(goodQuery).Build(),
			Metric("fail", "Failing Panel").Query(badQuery).Build(),
		),
	)

	results := Execute(context.Background(), ds, dash)

	if results == nil {
		t.Fatal("expected non-nil results")
	}
	if len(results.Panels) != 2 {
		t.Fatalf("expected 2 panel results, got %d", len(results.Panels))
	}

	// Good panel should succeed.
	okPR, ok := results.Panels["ok"]
	if !ok {
		t.Fatal("expected result for panel 'ok'")
	}
	if okPR.Error != nil {
		t.Errorf("panel 'ok': unexpected error: %v", okPR.Error)
	}
	if okPR.Data == nil {
		t.Error("panel 'ok': expected non-nil Data")
	}

	// Failing panel should have an error, nil Data.
	failPR, ok := results.Panels["fail"]
	if !ok {
		t.Fatal("expected result for panel 'fail'")
	}
	if failPR.Error == nil {
		t.Error("panel 'fail': expected non-nil Error")
	} else if !errors.Is(failPR.Error, sentinelErr) {
		t.Errorf("panel 'fail': expected error wrapping sentinelErr, got: %v", failPR.Error)
	}
}

// ---------------------------------------------------------------------------
// TestExecute_EmptyDashboard
// ---------------------------------------------------------------------------

func TestExecute_EmptyDashboard(t *testing.T) {
	t.Parallel()

	ds := &mockDataSource{
		results: map[string]*QueryResult{},
		errors:  map[string]error{},
	}

	dash := NewDashboard("Empty")

	results := Execute(context.Background(), ds, dash)

	if results == nil {
		t.Fatal("expected non-nil results")
	}
	if len(results.Panels) != 0 {
		t.Errorf("expected 0 panel results, got %d", len(results.Panels))
	}
}

// ---------------------------------------------------------------------------
// TestExecute_NoQuery
// ---------------------------------------------------------------------------

func TestExecute_NoQuery(t *testing.T) {
	t.Parallel()

	callCount := 0
	ds := &countingDataSource{callCountPtr: &callCount}

	dash := NewDashboard("Test",
		NewRow(
			// Panel without a query — should be skipped.
			Metric("nq", "No Query").Build(),
		),
	)

	results := Execute(context.Background(), ds, dash)

	if results == nil {
		t.Fatal("expected non-nil results")
	}
	if len(results.Panels) != 0 {
		t.Errorf("expected 0 panel results for panels without queries, got %d", len(results.Panels))
	}
	if callCount != 0 {
		t.Errorf("expected DataSource.Execute to not be called, but was called %d time(s)", callCount)
	}
}

// ---------------------------------------------------------------------------
// TestExecute_Duration
// ---------------------------------------------------------------------------

func TestExecute_Duration(t *testing.T) {
	t.Parallel()

	ds := &mockDataSource{
		results: map[string]*QueryResult{
			"SELECT 1": {Rows: []map[string]any{{"value": 1}}},
		},
		errors: map[string]error{},
	}

	dash := NewDashboard("Test",
		NewRow(
			Metric("p1", "P1").Query("SELECT 1").Build(),
		),
	)

	results := Execute(context.Background(), ds, dash)

	if results.Duration <= 0 {
		t.Error("expected positive total Duration")
	}

	pr := results.Panels["p1"]
	if pr == nil {
		t.Fatal("expected result for panel p1")
	}
	if pr.Duration <= 0 {
		t.Error("expected positive per-panel Duration")
	}
}

// ---------------------------------------------------------------------------
// TestExecute_MultipleRows
// ---------------------------------------------------------------------------

func TestExecute_MultipleRows(t *testing.T) {
	t.Parallel()

	q := "SELECT date, value FROM stats"
	ds := &mockDataSource{
		results: map[string]*QueryResult{
			q: {
				Columns: []QueryColumn{
					{Name: "date", Type: "string"},
					{Name: "value", Type: "number"},
				},
				Rows: []map[string]any{
					{"date": "2024-01", "value": float64(100)},
					{"date": "2024-02", "value": float64(200)},
					{"date": "2024-03", "value": float64(300)},
				},
			},
		},
		errors: map[string]error{},
	}

	dash := NewDashboard("Test",
		NewRow(
			Line("chart", "Stats").Query(q).Build(),
		),
	)

	results := Execute(context.Background(), ds, dash)

	pr := results.Panels["chart"]
	if pr == nil {
		t.Fatal("expected result for panel 'chart'")
	}
	if pr.Error != nil {
		t.Fatalf("unexpected error: %v", pr.Error)
	}
	if len(pr.Data.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(pr.Data.Rows))
	}
}

// ---------------------------------------------------------------------------
// countingDataSource — helper that tracks Execute call count
// ---------------------------------------------------------------------------

type countingDataSource struct {
	callCountPtr *int
}

func (c *countingDataSource) Execute(_ context.Context, _ string) (*QueryResult, error) {
	*c.callCountPtr++
	return &QueryResult{}, nil
}

func (c *countingDataSource) Close() error { return nil }
