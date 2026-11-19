package persistence_test

import (
	"context"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/logging/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/logging/permissions"
	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
	"github.com/stretchr/testify/require"
)

// documentReader configures the document resource the way a host such as Granite does: translated
// status and kind codes, captions from the document table and visibility by participation.
type documentReader struct {
	mu         sync.Mutex
	labelCalls [][]string
	visibility history.Visibility
}

func (d *documentReader) labels(_ context.Context, ids []string) (map[string]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.labelCalls = append(d.labelCalls, append([]string(nil), ids...))
	result := map[string]string{}
	for _, id := range ids {
		if id != "d6" { // d6 was deleted: its caption is only the one recorded with its events
			result[id] = "Letter №" + id[1:]
		}
	}
	return result, nil
}

func (d *documentReader) visible(context.Context) (history.Visibility, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.visibility, nil
}

func (d *documentReader) calls() [][]string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([][]string(nil), d.labelCalls...)
}

func documentDefinitions(d *documentReader) []history.Definition {
	kindLabel := func(_ context.Context, code string) string {
		if code == "incoming" {
			return "Incoming letter"
		}
		return ""
	}
	document := history.Resource{
		Name: "edo.document", LabelKey: "Documents.Resource", Labels: d.labels, Visible: d.visible,
		Fields: []history.Field{
			{Name: "status", LabelKey: "Documents.Status", Type: history.ValueString, ValueKeyPrefix: "Debts.Statuses"},
			{Name: "kind", LabelKey: "Documents.Kind", Type: history.ValueString, ValueLabel: kindLabel},
			{Name: "pages", LabelKey: "Documents.Pages", Type: history.ValueInteger},
		},
	}
	return []history.Definition{
		{
			Type: "edo.document.changed", Version: 1, Category: businessevent.CategoryBusiness,
			TitleKey: "Documents.History.Changed.Title", DescriptionKey: "Documents.History.Changed.Summary",
			Resources: []history.Resource{document},
		},
		{
			Type: "edo.document.employee_linked", Version: 1, Category: businessevent.CategoryBusiness,
			TitleKey: "Documents.History.Linked.Title", DescriptionKey: "Documents.History.Linked.Summary",
			Resources: []history.Resource{{Name: "edo.document"}, {Name: "employee"}},
		},
	}
}

type documentFixture struct {
	ctx     context.Context
	reader  *documentReader
	service *history.Service
}

// newDocumentFixture records a change of documents d1..d6 an hour apart (d6 newest and deleted since,
// so its provider caption is gone) and, last, a link of document d2 with employee 9.
func newDocumentFixture(t *testing.T, configure ...func(*history.Resource)) documentFixture {
	t.Helper()
	f := setupTest(t)
	reader := &documentReader{visibility: history.VisibleAll()}
	defs := documentDefinitions(reader)
	for _, c := range configure {
		defs = history.WithResource(defs, "edo.document", c)
	}
	registry := history.NewRegistry()
	require.NoError(t, registry.Register(append(defs, employeeV1())...))
	repo := persistence.NewBusinessEventRepository()
	writer := history.NewWriter(registry, repo)
	actor := createUser(t, f.Ctx, "Dilnoza", "Karimova", "", "history-documents@example.com")
	for i := 1; i <= 6; i++ {
		id := "d" + strconv.Itoa(i)
		_, err := writer.Record(asUser(f.Ctx, actor, "op-"+id), history.Input{
			Type: "edo.document.changed", Version: 1, Outcome: businessevent.OutcomeSucceeded,
			OccurredAt: historyStart.Add(time.Duration(i) * time.Hour),
			Subjects:   []businessevent.Subject{{Resource: "edo.document", ID: id, Label: "  Draft   №" + strconv.Itoa(i) + " "}},
			Changes: []businessevent.Change{{Resource: "edo.document", ID: id, Fields: []fielddiff.Diff{
				{Field: "status", Before: value(`"PENDING"`), After: value(`"ARCHIVED"`)},
				{Field: "kind", Before: fielddiff.Absent(), After: value(`"incoming"`)},
				{Field: "pages", Before: value(`2`), After: value(`3`)},
			}}},
		})
		require.NoError(t, err)
	}
	_, err := writer.Record(asUser(f.Ctx, actor, "op-link"), history.Input{
		Type: "edo.document.employee_linked", Version: 1, Outcome: businessevent.OutcomeSucceeded,
		OccurredAt: historyStart.Add(10 * time.Hour),
		Subjects:   []businessevent.Subject{{Resource: "edo.document", ID: "d2"}, {Resource: "employee", ID: "9"}},
	})
	require.NoError(t, err)
	return documentFixture{ctx: f.Ctx, reader: reader, service: history.NewService(registry, repo)}
}

func (d documentFixture) restrict(v history.Visibility) {
	d.reader.mu.Lock()
	defer d.reader.mu.Unlock()
	d.reader.visibility = v
}

func subjectIDs(result history.Result) []string {
	var ids []string
	for _, item := range result.Items {
		for _, s := range item.Subjects {
			if s.Resource == "edo.document" {
				ids = append(ids, s.ID)
			}
		}
	}
	return ids
}

func TestHistoryService_TranslatesDeclaredValueCodes(t *testing.T) {
	t.Parallel()
	d := newDocumentFixture(t)
	result, err := d.service.History(d.ctxReader(), history.Query{Scope: history.Scope{Kind: history.ScopeEntity, Resource: "edo.document", ID: "d1"}})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	require.Len(t, item.Display, 1)
	require.Equal(t, item.Changes[0].Fields[0].Field, item.Display[0].Fields[0].Field, "Display follows the order of Changes")

	fields := map[string]history.FieldDisplay{}
	for _, f := range item.Display[0].Fields {
		fields[f.Field] = f
	}
	// A translation key per code: the page and exports show the translation and keep the code.
	require.Equal(t, history.ValueDisplay{State: fielddiff.StateValue, Text: "Pending", Code: "PENDING"}, fields["status"].Before)
	// A code without a translation reads as stored.
	require.Equal(t, history.ValueDisplay{State: fielddiff.StateValue, Text: "ARCHIVED"}, fields["status"].After)
	// A translation function.
	require.Equal(t, history.ValueDisplay{State: fielddiff.StateAbsent}, fields["kind"].Before)
	require.Equal(t, history.ValueDisplay{State: fielddiff.StateValue, Text: "Incoming letter", Code: "incoming"}, fields["kind"].After)
	// Fields without a declared translation are unchanged.
	require.Equal(t, history.ValueDisplay{State: fielddiff.StateValue, Text: "3"}, fields["pages"].After)
	require.Equal(t, "pages", fields["pages"].Label, "an untranslated field label falls back to the field name")
}

func TestHistoryService_CaptionsSubjectsWithOneBatchedCallPerPage(t *testing.T) {
	t.Parallel()
	d := newDocumentFixture(t)
	result, err := d.service.History(d.ctxReader(), history.Query{Scope: history.Scope{Kind: history.ScopeResource, Resource: "edo.document"}})
	require.NoError(t, err)
	require.Len(t, result.Items, 7)

	calls := d.reader.calls()
	require.Len(t, calls, 1, "one provider call for the whole page, not one per row")
	require.ElementsMatch(t, []string{"d1", "d2", "d3", "d4", "d5", "d6"}, calls[0])

	labels := map[string]history.Subject{}
	for _, item := range result.Items {
		for _, s := range item.Subjects {
			labels[s.Resource+":"+s.ID] = s
		}
	}
	require.Equal(t, "Letter №1", labels["edo.document:d1"].Name)
	require.Equal(t, "edo.document: Letter №1", labels["edo.document:d1"].Label)
	// The deleted document keeps the caption recorded with its event, whitespace normalised.
	require.Equal(t, "Draft №6", labels["edo.document:d6"].Name)
	// A resource without a provider reads as before.
	require.Equal(t, history.Subject{Resource: "employee", ID: "9", Label: "employee #9"}, labels["employee:9"])
	// Change sections carry the same caption.
	for _, item := range result.Items {
		for _, c := range item.Display {
			require.Equal(t, labels[c.Resource+":"+c.ID].Label, c.Subject)
		}
	}

	subject, ok := d.service.Subject(d.ctxReader(), "edo.document", "d3")
	require.True(t, ok)
	require.Equal(t, "Letter №3", subject.Name)
}

func TestHistoryService_VisibleHidesEventsBeforePagination(t *testing.T) {
	t.Parallel()
	d := newDocumentFixture(t)

	tests := []struct {
		name       string
		visibility history.Visibility
		visible    []string
	}{
		{"listed objects", history.VisibleIDs("d1", "d3", "d5"), []string{"d5", "d3", "d1"}},
		// The condition's own $1 does not collide with the reader's arguments.
		{"SQL condition", history.VisibleWhere(`{id} = ANY($1::text[]) AND {id} <> $2`, []string{"d2", "d4", "d6"}, "d6"), []string{"d2", "d4", "d2"}},
		{"both", history.Visibility{IDs: []string{"d1"}, Condition: `{id} = $1`, Args: []any{"d6"}}, []string{"d6", "d1"}},
		{"none", history.Visibility{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d.restrict(tt.visibility)
			ctx := d.ctxReader()
			var seen []string
			query := history.Query{Scope: all(), Limit: 2}
			for page := 0; ; page++ {
				require.Less(t, page, 10)
				result, err := d.service.History(ctx, query)
				require.NoError(t, err)
				// A page is never short because of hidden rows, and a cursor only appears when
				// visible rows remain.
				if result.NextCursor != nil {
					require.Len(t, result.Items, 2)
				}
				for _, item := range result.Items {
					if item.EventType == "edo.document.employee_linked" {
						require.Contains(t, tt.visible, "d2", "a link is shown only with its document")
					}
				}
				seen = append(seen, subjectIDs(result)...)
				if result.NextCursor == nil {
					break
				}
				query.Cursor = *result.NextCursor
			}
			require.Equal(t, tt.visible, seen)

			_, ok := d.service.Subject(ctx, "edo.document", "d4")
			require.Equal(t, slices.Contains(tt.visible, "d4"), ok, "a hidden object's caption is not disclosed")
			entity, err := d.service.History(ctx, history.Query{Scope: history.Scope{Kind: history.ScopeEntity, Resource: "edo.document", ID: "d4"}})
			require.NoError(t, err)
			require.Equal(t, slices.Contains(tt.visible, "d4"), len(entity.Items) > 0)
		})
	}

	// The employee side of a link to a hidden document is not shown either.
	d.restrict(history.VisibleIDs("d1"))
	result, err := d.service.History(d.ctxReader(), history.Query{Scope: history.Scope{Kind: history.ScopeEntity, Resource: "employee", ID: "9"}})
	require.NoError(t, err)
	require.Empty(t, result.Items)
}

func TestHistoryService_VisibleAppliesToUnreadableCompanionResources(t *testing.T) {
	t.Parallel()
	// The reader may not read documents at all, but may read employees: the link of employee 9 to
	// document d2 stays visible only while d2 is.
	d := newDocumentFixture(t, func(r *history.Resource) { r.ReadPermission = &contractRead })
	query := history.Query{Scope: history.Scope{Kind: history.ScopeEntity, Resource: "employee", ID: "9"}}

	result, err := d.service.History(d.ctxReader(), query)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, []history.Subject{{Resource: "employee", ID: "9", Label: "employee #9"}}, result.Items[0].Subjects)

	d.restrict(history.VisibleIDs("d1"))
	result, err = d.service.History(d.ctxReader(), query)
	require.NoError(t, err)
	require.Empty(t, result.Items)
}

func TestHistoryService_VisibleConditionOnlySeesItsOwnResource(t *testing.T) {
	t.Parallel()
	d := newDocumentFixture(t)
	// The cast fails on any ID that is not "d<number>", e.g. the employee "9" of the link event,
	// so the read only succeeds when the condition is never applied to other resources.
	d.restrict(history.VisibleWhere(`substr({id}, 2)::int = ANY($1::int[]) AND left({id}, 1) = 'd'`, []int{2, 3}))
	result, err := d.service.History(d.ctxReader(), history.Query{Scope: all()})
	require.NoError(t, err)
	require.Equal(t, []string{"d2", "d3", "d2"}, subjectIDs(result))
}

func TestHistoryService_VisibilityErrorFailsTheRead(t *testing.T) {
	t.Parallel()
	d := newDocumentFixture(t)
	d.restrict(history.VisibleWhere(`{id} = $3`, "d1"))
	_, err := d.service.History(d.ctxReader(), history.Query{Scope: all()})
	require.Error(t, err, "an undeclared placeholder must fail instead of reading another argument")
}

func (d documentFixture) ctxReader() context.Context {
	return documentFixtureReader(d.ctx)
}

func documentFixtureReader(ctx context.Context) context.Context {
	return historyFixture{ctx: ctx}.as(permissions.ViewLogs, salaryRead)
}
