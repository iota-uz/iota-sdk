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
			wantErr: `application.normalizeRuntimeTags: invalid runtime tag "unknown"`,
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

	tests := []struct {
		name         string
		registration RuntimeRegistration
		activeTags   map[RuntimeTag]struct{}
		expected     bool
	}{
		{
			name: "component without tags applies to all",
			registration: RuntimeRegistration{
				Component: noopRuntimeComponent{name: "all"},
			},
			activeTags: runtimeTagSet([]RuntimeTag{RuntimeTagAPI, RuntimeTagWorker}),
			expected:   true,
		},
		{
			name: "api-tagged component applies to api runtime",
			registration: RuntimeRegistration{
				Component: noopRuntimeComponent{name: "api"},
				Tags:      []RuntimeTag{RuntimeTagAPI},
			},
			activeTags: runtimeTagSet([]RuntimeTag{RuntimeTagAPI}),
			expected:   true,
		},
		{
			name: "worker-tagged component does not apply to api runtime",
			registration: RuntimeRegistration{
				Component: noopRuntimeComponent{name: "worker"},
				Tags:      []RuntimeTag{RuntimeTagWorker},
			},
			activeTags: runtimeTagSet([]RuntimeTag{RuntimeTagAPI}),
			expected:   false,
		},
		{
			name: "unknown tag does not match active runtime",
			registration: RuntimeRegistration{
				Component: noopRuntimeComponent{name: "none"},
				Tags:      []RuntimeTag{"maintenance"},
			},
			activeTags: runtimeTagSet([]RuntimeTag{RuntimeTagAPI, RuntimeTagWorker}),
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, tt.registration.AppliesTo(tt.activeTags))
		})
	}
}

func TestApplication_StartRuntimeUsesTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		tags                []RuntimeTag
		wantAPIStarts       int
		wantWorkerStarts    int
		wantDuplicateErrMsg string
	}{
		{
			name:             "api only",
			tags:             []RuntimeTag{RuntimeTagAPI},
			wantAPIStarts:    1,
			wantWorkerStarts: 0,
		},
		{
			name:             "worker only",
			tags:             []RuntimeTag{RuntimeTagWorker},
			wantAPIStarts:    0,
			wantWorkerStarts: 1,
		},
		{
			name:                "api and worker",
			tags:                []RuntimeTag{RuntimeTagWorker, RuntimeTagAPI},
			wantAPIStarts:       1,
			wantWorkerStarts:    1,
			wantDuplicateErrMsg: "runtime already started with tags [api, worker]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			apiComponent := &recordingRuntimeComponent{name: "api"}
			workerComponent := &recordingRuntimeComponent{name: "worker"}
			app := &application{
				runtimeComponents: []RuntimeRegistration{
					{Component: apiComponent, Tags: []RuntimeTag{RuntimeTagAPI}},
					{Component: workerComponent, Tags: []RuntimeTag{RuntimeTagWorker}},
				},
			}

			err := app.StartRuntime(context.Background(), tt.tags...)
			require.NoError(t, err)
			assert.Equal(t, tt.wantAPIStarts, apiComponent.started)
			assert.Equal(t, tt.wantWorkerStarts, workerComponent.started)

			err = app.StartRuntime(context.Background(), tt.tags...)
			require.NoError(t, err)
			assert.Equal(t, tt.wantAPIStarts, apiComponent.started)
			assert.Equal(t, tt.wantWorkerStarts, workerComponent.started)

			if tt.wantDuplicateErrMsg != "" {
				err = app.StartRuntime(context.Background(), RuntimeTagAPI)
				require.EqualError(t, err, tt.wantDuplicateErrMsg)
			}
		})
	}
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
