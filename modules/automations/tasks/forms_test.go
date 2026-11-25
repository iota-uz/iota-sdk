package tasks_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations/tasks"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
)

type form string

func (f form) Kind() string                                         { return string(f) }
func (form) Render(context.Context, tasks.TaskView) templ.Component { return templ.NopComponent }
func (form) Decide(context.Context, tasks.TaskView, user.User, json.RawMessage) (tasks.FormDecision, error) {
	return tasks.FormDecision{}, nil
}

func TestFormKindOf(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw  string
		kind string
		ok   bool
	}{
		{``, "", true},
		{`{"type":"object"}`, "", true},
		{`{"type":"object","x-iota-form":" eimzo.sign "}`, "eimzo.sign", true},
		{`{"x-iota-form":""}`, "", false},
		{`{"x-iota-form":7}`, "", false},
	} {
		kind, ok := tasks.FormKindOf(json.RawMessage(tc.raw))
		assert.Equal(t, tc.kind, kind, tc.raw)
		assert.Equal(t, tc.ok, ok, tc.raw)
	}
}

func TestFormsRegistry(t *testing.T) {
	t.Parallel()
	forms := tasks.NewForms(form("a"), form("b"))
	require.NoError(t, forms.Check())
	f, ok, err := forms.Form("b")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "b", f.Kind())
	_, ok, err = forms.Form("c")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, []string{"a", "b"}, forms.Kinds())

	dup := tasks.NewForms(form("a"), form("a"))
	require.Error(t, dup.Check(), "a kind twice fails")
	_, _, err = dup.Form("a")
	require.Error(t, err)
}
