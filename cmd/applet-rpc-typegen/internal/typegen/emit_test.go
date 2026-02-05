package typegen

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/applet"
	"github.com/stretchr/testify/require"
)

func TestEmitTypeScript(t *testing.T) {
	t.Parallel()

	desc := &applet.TypedRouterDescription{
		Methods: []applet.TypedMethodDescription{
			{
				Name:   "demo.ping",
				Params: applet.TypeRef{Kind: "named", Name: "PingParams"},
				Result: applet.TypeRef{Kind: "named", Name: "PingResult"},
			},
		},
		Types: map[string]applet.TypedTypeObject{
			"PingParams": {Fields: []applet.TypedField{}},
			"PingResult": {Fields: []applet.TypedField{{Name: "ok", Type: applet.TypeRef{Kind: "boolean"}}}},
		},
	}

	out, err := EmitTypeScript(desc, "DemoRPC")
	require.NoError(t, err)
	require.Contains(t, out, `export type DemoRPC`)
	require.Contains(t, out, `"demo.ping": { params: PingParams; result: PingResult }`)
	require.Contains(t, out, `export interface PingResult`)
	require.Contains(t, out, `ok: boolean`)
}
