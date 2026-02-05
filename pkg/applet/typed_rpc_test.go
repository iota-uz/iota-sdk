package applet

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type demoParams struct {
	ID string `json:"id"`
}

type demoResult struct {
	Ok bool `json:"ok"`
}

func TestTypedRPCRouter_ConfigAndDecode(t *testing.T) {
	t.Parallel()

	r := NewTypedRPCRouter()
	AddProcedure(r, "demo.ok", Procedure[demoParams, demoResult]{
		RequirePermissions: []string{"demo.access"},
		Handler: func(ctx context.Context, params demoParams) (demoResult, error) {
			if params.ID == "" {
				return demoResult{}, serrors.E(serrors.Op("demo"), serrors.KindValidation, "id required")
			}
			return demoResult{Ok: true}, nil
		},
	})

	cfg := r.Config()
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Methods)
	m, ok := cfg.Methods["demo.ok"]
	require.True(t, ok)
	assert.Equal(t, []string{"demo.access"}, m.RequirePermissions)

	t.Run("ValidPayload", func(t *testing.T) {
		out, err := m.Handler(context.Background(), json.RawMessage(`{"id":"1"}`))
		require.NoError(t, err)
		assert.Equal(t, demoResult{Ok: true}, out)
	})

	t.Run("InvalidPayload", func(t *testing.T) {
		_, err := m.Handler(context.Background(), json.RawMessage(`{"id":""}`))
		require.Error(t, err)
	})
}

func TestDescribeTypedRPCRouter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
	}{
		{name: "DescribeMethodsPresent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewTypedRPCRouter()
			AddProcedure(r, "demo.ok", Procedure[demoParams, demoResult]{
				Handler: func(ctx context.Context, params demoParams) (demoResult, error) {
					return demoResult{Ok: true}, nil
				},
			})

			desc, err := DescribeTypedRPCRouter(r)
			require.NoError(t, err)
			require.NotNil(t, desc)
			require.Len(t, desc.Methods, 1)
			assert.Equal(t, "demo.ok", desc.Methods[0].Name)
			require.NotEmpty(t, desc.Types)
		})
	}
}
