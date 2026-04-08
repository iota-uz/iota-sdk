package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeRuntimeTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []RuntimeTag
		expected []RuntimeTag
		wantErr  string
	}{
		{
			name:     "deduplicates and sorts tags",
			input:    []RuntimeTag{RuntimeTagWorker, RuntimeTagAPI, RuntimeTagWorker},
			expected: []RuntimeTag{RuntimeTagAPI, RuntimeTagWorker},
		},
		{
			name:    "rejects unknown tags",
			input:   []RuntimeTag{"unknown"},
			wantErr: `invalid runtime tag "unknown"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeRuntimeTags(tt.input)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestRuntimeRegistration_AppliesTo(t *testing.T) {
	t.Parallel()

	activeTags := runtimeTagSet([]RuntimeTag{RuntimeTagAPI, RuntimeTagWorker})

	assert.True(t, RuntimeRegistration{
		Component: noopRuntimeComponent{name: "all"},
	}.AppliesTo(activeTags))
	assert.True(t, RuntimeRegistration{
		Component: noopRuntimeComponent{name: "api"},
		Tags:      []RuntimeTag{RuntimeTagAPI},
	}.AppliesTo(activeTags))
	assert.False(t, RuntimeRegistration{
		Component: noopRuntimeComponent{name: "none"},
		Tags:      []RuntimeTag{"maintenance"},
	}.AppliesTo(activeTags))
}

func TestApplication_StartRuntimeUsesTags(t *testing.T) {
	t.Parallel()

	apiComponent := &recordingRuntimeComponent{name: "api"}
	workerComponent := &recordingRuntimeComponent{name: "worker"}
	app := &application{
		runtimeComponents: []RuntimeRegistration{
			{Component: apiComponent, Tags: []RuntimeTag{RuntimeTagAPI}},
			{Component: workerComponent, Tags: []RuntimeTag{RuntimeTagWorker}},
		},
	}

	err := app.StartRuntime(context.Background(), RuntimeTagWorker, RuntimeTagAPI)
	require.NoError(t, err)
	assert.Equal(t, 1, apiComponent.started)
	assert.Equal(t, 1, workerComponent.started)

	err = app.StartRuntime(context.Background(), RuntimeTagAPI, RuntimeTagWorker)
	require.NoError(t, err)
	assert.Equal(t, 1, apiComponent.started)
	assert.Equal(t, 1, workerComponent.started)

	err = app.StartRuntime(context.Background(), RuntimeTagAPI)
	require.EqualError(t, err, "runtime already started with tags [api, worker]")
}

type noopRuntimeComponent struct {
	name string
}

func (c noopRuntimeComponent) Name() string              { return c.name }
func (noopRuntimeComponent) Start(context.Context) error { return nil }
func (noopRuntimeComponent) Stop(context.Context) error  { return nil }

type recordingRuntimeComponent struct {
	name    string
	started int
	stopped int
}

func (c *recordingRuntimeComponent) Name() string { return c.name }

func (c *recordingRuntimeComponent) Start(context.Context) error {
	c.started++
	return nil
}

func (c *recordingRuntimeComponent) Stop(context.Context) error {
	c.stopped++
	return nil
}
