package frame

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromRecords_PreservesTimeCells(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	type record struct {
		At       time.Time  `json:"at"`
		Optional *time.Time `json:"optional"`
	}
	frames, err := FromRecords("records", []record{{At: now, Optional: &now}})
	require.NoError(t, err)
	fr := frames.Primary()
	at, ok := fr.Field("at")
	require.True(t, ok)
	require.Equal(t, FieldTypeTime, at.Type)
	require.Equal(t, now, at.Values[0])
	optional, ok := fr.Field("optional")
	require.True(t, ok)
	require.Equal(t, FieldTypeTime, optional.Type)
	require.Equal(t, now, optional.Values[0])
}

func TestFrameClone_ClonesHyperlinkPointers(t *testing.T) {
	t.Parallel()
	link := &Hyperlink{URL: "https://example.test", Label: "original"}
	fr, err := New("links", Field{Name: "link", Values: []any{link}})
	require.NoError(t, err)
	cloned := fr.Clone()
	clonedLink := cloned.Fields[0].Values[0].(*Hyperlink)
	clonedLink.Label = "changed"
	require.Equal(t, "original", link.Label)
}

func TestNormalizeRejectsUnevenFields(t *testing.T) {
	t.Parallel()

	fr := &Frame{
		Name: "broken",
		Fields: []Field{
			{Name: "label", Values: []any{"a", "b"}},
			{Name: "value", Values: []any{1}},
		},
	}
	require.Error(t, fr.Normalize())
}

func TestRowsRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	fr, err := New("report",
		Field{Name: "label", Values: []any{"one", "two"}},
		Field{Name: "value", Values: []any{1.0, 2.0}},
		Field{Name: "at", Values: []any{now, now.Add(time.Hour)}},
	)
	require.NoError(t, err)

	rows := fr.Rows()
	require.Len(t, rows, 2)
	assert.Equal(t, "one", rows[0]["label"])
	assert.InDelta(t, 2.0, rows[1]["value"].(float64), 0.001)
	assert.Equal(t, now.Add(time.Hour), rows[1]["at"])
}

func TestAppendRowBootstrapsFields(t *testing.T) {
	t.Parallel()

	fr := &Frame{Name: "dynamic"}
	require.NoError(t, fr.AppendRow(map[string]any{"label": "Revenue", "value": 42.0}))
	require.Equal(t, 1, fr.RowCount)

	field, ok := fr.Field("label")
	require.True(t, ok)
	require.Equal(t, "Revenue", field.Values[0])
}

func TestClonePreservesFieldLookups(t *testing.T) {
	t.Parallel()

	fr, err := New("sales",
		Field{Name: "label", Type: FieldTypeString, Values: []any{"OSAGO"}},
		Field{Name: "value", Type: FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	cloned := fr.Clone()
	field, ok := cloned.Field("label")
	require.True(t, ok)
	require.NotNil(t, field)
	require.Equal(t, "label", field.Name)
}

func TestFieldLazyIndexBuild(t *testing.T) {
	t.Parallel()

	fr := &Frame{
		Name: "lazy",
		Fields: []Field{
			{Name: "a", Values: []any{"x"}},
			{Name: "b", Values: []any{1}},
		},
	}
	// Field() should work even without prior Normalize() call
	field, ok := fr.Field("a")
	require.True(t, ok)
	require.Equal(t, "a", field.Name)
}
