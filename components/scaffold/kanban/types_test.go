package kanban

import (
	"net/url"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func TestColumnKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values url.Values
		want   string
	}{
		{
			name: "prefers canonical query param",
			values: url.Values{
				QueryParamColumn:       []string{"approved"},
				LegacyQueryParamColumn: []string{"rejected"},
			},
			want: "approved",
		},
		{
			name: "falls back to legacy drag param",
			values: url.Values{
				LegacyQueryParamColumn: []string{"waiting-approval"},
			},
			want: "waiting-approval",
		},
		{
			name: "empty when no column key is present",
			values: url.Values{
				FormParamCardKey: []string{"card-1"},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ColumnKey(tt.values); got != tt.want {
				t.Fatalf("ColumnKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanQueryParams(t *testing.T) {
	t.Parallel()

	values := url.Values{
		"Search":                []string{"GULCHEHRA"},
		"view":                  []string{"kanban"},
		QueryParamColumn:        []string{"approved"},
		LegacyQueryParamColumn:  []string{"rejected"},
		queryParamPage:          []string{"4"},
		queryParamLimit:         []string{"20"},
		FormParamColumnOldIndex: []string{"0"},
		FormParamColumnNewIndex: []string{"1"},
		FormParamCardKey:        []string{"card-1"},
		FormParamCardOldColumn:  []string{"draft"},
		FormParamCardNewColumn:  []string{"approved"},
		FormParamCardOldIndex:   []string{"2"},
		FormParamCardNewIndex:   []string{"3"},
		"Statuses":              []string{"1", "2"},
	}

	got := CleanQueryParams(values)
	for _, key := range transportQueryParams {
		if _, ok := got[key]; ok {
			t.Fatalf("CleanQueryParams() kept transport param %q in %v", key, got)
		}
	}
	if got.Get("Search") != "GULCHEHRA" {
		t.Fatalf("CleanQueryParams() dropped Search: %v", got)
	}
	if got.Get("view") != "kanban" {
		t.Fatalf("CleanQueryParams() dropped view: %v", got)
	}
	if vals := got["Statuses"]; len(vals) != 2 || vals[0] != "1" || vals[1] != "2" {
		t.Fatalf("CleanQueryParams() changed multi-value filter: %v", got)
	}
	if values.Get(QueryParamColumn) != "approved" {
		t.Fatalf("CleanQueryParams() mutated input values: %v", values)
	}
}

func TestNextColumnChunkURLCleansTransportParams(t *testing.T) {
	t.Parallel()

	got := nextColumnChunkURL("/kanban/cards", "approved", &ColumnLoadState{
		NextPage: 2,
		PerPage:  20,
	}, url.Values{
		"Search":               []string{"GULCHEHRA"},
		LegacyQueryParamColumn: []string{""},
		FormParamCardKey:       []string{""},
		FormParamCardOldColumn: []string{"draft"},
		queryParamPage:         []string{"1"},
		queryParamLimit:        []string{"10"},
	})

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("nextColumnChunkURL() returned invalid URL %q: %v", got, err)
	}

	query := parsed.Query()
	if query.Get("Search") != "GULCHEHRA" {
		t.Fatalf("nextColumnChunkURL() dropped Search: %s", got)
	}
	if query.Get(QueryParamColumn) != "approved" {
		t.Fatalf("nextColumnChunkURL() did not set column: %s", got)
	}
	if query.Get(queryParamPage) != "2" || query.Get(queryParamLimit) != "20" {
		t.Fatalf("nextColumnChunkURL() did not set pagination: %s", got)
	}
	if query.Has(LegacyQueryParamColumn) || query.Has(FormParamCardKey) || query.Has(FormParamCardOldColumn) {
		t.Fatalf("nextColumnChunkURL() kept drag params: %s", got)
	}
}

func TestCardSortConfig(t *testing.T) {
	t.Parallel()

	hookExpr := "window.boardGuard"

	tests := []struct {
		name             string
		beforeCardChange string
		assert           func(t *testing.T, got string)
	}{
		{
			name: "keeps the default flow without a hook",
			assert: func(t *testing.T, got string) {
				want := `{
		delayOnTouchOnly: true,
		delay: 150,
		touchStartThreshold: 8,
		onEnd: (event) => {
			changeCard({
				key: event.item.dataset.cardKey,
				newCol: event.to.dataset.colKey,
				oldCol: event.from.dataset.colKey,
				oldIndex: event.oldIndex,
				newIndex: event.newIndex
			})
			$nextTick(() => htmx.trigger("#kanban-card-trigger-board", 'cardChanged'));
		}
	}`
				if got != want {
					t.Fatalf("cardSortConfig() mismatch:\ngot:  %s\nwant: %s", got, want)
				}
			},
		},
		{
			name:             "invokes the hook with the transition context",
			beforeCardChange: hookExpr,
			assert: func(t *testing.T, got string) {
				for _, want := range []string{
					`(window.boardGuard)(ctx)`,
					"cardKey: event.item.dataset.cardKey",
					"oldCol: event.from.dataset.colKey",
					"newCol: event.to.dataset.colKey",
					"oldIndex: event.oldIndex",
					"newIndex: event.newIndex",
				} {
					if !strings.Contains(got, want) {
						t.Fatalf("cardSortConfig() missing %q:\n%s", want, got)
					}
				}
			},
		},
		{
			name:             "rolls back and skips the post when the hook returns false",
			beforeCardChange: hookExpr,
			assert: func(t *testing.T, got string) {
				for _, want := range []string{
					"const origin = event.item.__kanbanOrigin;",
					"delete event.item.__kanbanOrigin;",
					"origin.next.parentNode === origin.container",
					"insertBefore(event.item, ref)",
					"if (decision === false)",
					"typeof decision.then === 'function'",
				} {
					if !strings.Contains(got, want) {
						t.Fatalf("cardSortConfig() missing %q:\n%s", want, got)
					}
				}
			},
		},
		{
			name:             "still triggers the card change post when allowed",
			beforeCardChange: hookExpr,
			assert: func(t *testing.T, got string) {
				for _, want := range []string{
					`htmx.trigger("#kanban-card-trigger-board", 'cardChanged')`,
					"changeCard(ctx)",
				} {
					if !strings.Contains(got, want) {
						t.Fatalf("cardSortConfig() missing %q:\n%s", want, got)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := cardSortConfig(CardTriggerID("board"), tt.beforeCardChange)
			tt.assert(t, got)
		})
	}
}

func TestConfigWithBeforeCardChange(t *testing.T) {
	t.Parallel()

	board := NewBoard[Card]("board", "Board", NewColumn[Card]("a", templ.Raw("A")))
	cfg := NewConfig(board, "/columns", "/cards")

	if cfg.BeforeCardChange != "" {
		t.Fatalf("NewConfig() set BeforeCardChange to %q, want empty", cfg.BeforeCardChange)
	}

	got := cfg.WithBeforeCardChange("window.boardGuard")
	if got != cfg {
		t.Fatalf("WithBeforeCardChange() returned a new config, want the same pointer")
	}
	if cfg.BeforeCardChange != "window.boardGuard" {
		t.Fatalf("WithBeforeCardChange() set BeforeCardChange to %q", cfg.BeforeCardChange)
	}

	cfg.WithBeforeCardChange("")
	if cfg.BeforeCardChange != "" {
		t.Fatalf("WithBeforeCardChange(\"\") left BeforeCardChange as %q", cfg.BeforeCardChange)
	}
}
