package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type staticSource struct {
	name        string
	definitions []Definition
	err         error
}

func (s staticSource) Name() string { return s.name }
func (s staticSource) Definitions(context.Context) ([]Definition, error) {
	return s.definitions, s.err
}

func runtimeDefinition(key, ref string) Definition {
	return Definition{
		Module: "ext", Key: key, Source: "ts", SnapshotRef: ref,
		Name: map[string]string{"en": "Runtime event"},
	}
}

func TestCatalogRegisterSource_RegistersRuntimeDefinitions(t *testing.T) {
	c := NewCatalog()
	require.NoError(t, c.Register(TestDefinition()))
	require.NoError(t, c.RegisterSource(context.Background(), staticSource{name: "ts", definitions: []Definition{runtimeDefinition("ext.thing.happened.v1", "snap-1")}}))

	d, ok := c.Get("ext.thing.happened.v1")
	require.True(t, ok)
	require.Equal(t, "ts", d.Source)
	require.Equal(t, "snap-1", d.SnapshotRef)
	require.Len(t, c.ByModule("ext"), 1)
	require.Len(t, c.Definitions(), 2)
}

func TestCatalogRegisterSource_Rejections(t *testing.T) {
	tests := []struct {
		name   string
		source staticSource
	}{
		{"wrong source", staticSource{name: "ts", definitions: []Definition{{Module: "ext", Key: "ext.a.b.v1", Name: map[string]string{"en": "x"}, Source: "other", SnapshotRef: "s"}}}},
		{"missing snapshot", staticSource{name: "ts", definitions: []Definition{{Module: "ext", Key: "ext.a.b.v1", Name: map[string]string{"en": "x"}, Source: "ts"}}}},
		{"duplicate inside source", staticSource{name: "ts", definitions: []Definition{runtimeDefinition("ext.a.b.v1", "s"), runtimeDefinition("ext.a.b.v1", "s")}}},
		{"duplicates native key", staticSource{name: "ts", definitions: []Definition{runtimeDefinition(TestEventKey, "s")}}},
		{"unversioned key", staticSource{name: "ts", definitions: []Definition{runtimeDefinition("ext.a.b", "s")}}},
		{"source failure", staticSource{name: "ts", err: errors.New("boom")}},
		{"empty source name", staticSource{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCatalog()
			require.NoError(t, c.Register(TestDefinition()))
			require.Error(t, c.RegisterSource(context.Background(), tt.source))
			require.Len(t, c.Definitions(), 1, "a failed source registers nothing")
		})
	}
}

func TestCatalogRegister_RejectsRuntimeMetadata(t *testing.T) {
	require.Error(t, NewCatalog().Register(runtimeDefinition("ext.a.b.v1", "s")))
}

func TestCatalogRegisterSource_NoSilentOverwrite(t *testing.T) {
	c := NewCatalog()
	ctx := context.Background()
	require.NoError(t, c.RegisterSource(ctx, staticSource{name: "ts", definitions: []Definition{runtimeDefinition("ext.a.b.v1", "snap-1")}}))
	require.Error(t, c.RegisterSource(ctx, staticSource{name: "ts", definitions: []Definition{runtimeDefinition("ext.a.b.v1", "snap-2")}}))
	// An update ships as a new version of the key.
	require.NoError(t, c.RegisterSource(ctx, staticSource{name: "ts", definitions: []Definition{runtimeDefinition("ext.a.b.v2", "snap-2")}}))
	d, _ := c.Get("ext.a.b.v1")
	require.Equal(t, "snap-1", d.SnapshotRef)
}

func TestCatalogDeactivate_KeepsPendingSnapshotResolvable(t *testing.T) {
	c := NewCatalog()
	ctx := context.Background()
	require.NoError(t, c.Register(TestDefinition()))
	require.NoError(t, c.RegisterSource(ctx, staticSource{name: "ts", definitions: []Definition{runtimeDefinition("ext.a.b.v1", "snap-1")}}))
	pending := Event{Key: "ext.a.b.v1", ID: "e1", TenantID: uuid.New(), SnapshotRef: "snap-1"}

	c.Deactivate("ts")

	_, active := c.Get("ext.a.b.v1")
	require.False(t, active, "deactivated definition leaves the settings catalog")
	require.Len(t, c.Definitions(), 1)
	_, err := c.Normalize(Event{Key: "ext.a.b.v1", ID: "e2", TenantID: uuid.New()})
	require.Error(t, err, "new events without a snapshot are rejected")
	normalized, err := c.Normalize(pending)
	require.NoError(t, err, "pending delivery still resolves its snapshot")
	require.Equal(t, "snap-1", normalized.SnapshotRef)
	d, ok := c.For(pending)
	require.True(t, ok)
	require.Equal(t, "snap-1", d.SnapshotRef)

	_, ok = c.For(Event{Key: "ext.a.b.v1", SnapshotRef: "unknown"})
	require.False(t, ok)
	_, ok = c.For(Event{Key: TestEventKey})
	require.True(t, ok, "native definitions are untouched by Deactivate")
}

func TestEvent_SnapshotRefSurvivesJSONRoundTrip(t *testing.T) {
	data, err := json.Marshal(Event{Key: "ext.a.b.v1", SnapshotRef: "snap-1"})
	require.NoError(t, err)
	var decoded Event
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, "snap-1", decoded.SnapshotRef)

	data, err = json.Marshal(Event{Key: TestEventKey})
	require.NoError(t, err)
	require.NotContains(t, string(data), "snapshot_ref")
}
