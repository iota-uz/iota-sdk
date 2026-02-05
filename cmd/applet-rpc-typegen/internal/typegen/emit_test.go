package typegen

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/applet"
	"github.com/stretchr/testify/require"
)

func TestEmitTypeScript(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		desc         *applet.TypedRouterDescription
		typeName     string
		wantContains []string
		wantErr      bool
	}{
		{
			name: "SingleMethod",
			desc: &applet.TypedRouterDescription{
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
			},
			typeName: "DemoRPC",
			wantContains: []string{
				`export type DemoRPC`,
				`"demo.ping": { params: PingParams; result: PingResult }`,
				`export type PingParams = Record<string, never>`,
				`export interface PingResult`,
				`ok: boolean`,
			},
		},
		{
			name:     "NilDescription",
			desc:     nil,
			typeName: "DemoRPC",
			wantErr:  true,
		},
		{
			name:     "EmptyTypeName",
			desc:     &applet.TypedRouterDescription{},
			typeName: "",
			wantErr:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out, err := EmitTypeScript(tc.desc, tc.typeName)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			for _, want := range tc.wantContains {
				require.Contains(t, out, want)
			}
		})
	}
}
