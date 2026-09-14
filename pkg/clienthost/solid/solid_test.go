package solid

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type props struct {
	Name string `json:"name"`
}

func TestImportBuildsStableDescriptorAndRender(t *testing.T) {
	t.Parallel()
	component := Import[props]("./Screen.tsx", WithRPC(
		Query[props, props]("example.load", Cacheable(3)),
		Action[props, props]("example.save", Invalidates("example.load")),
	))
	require.Equal(t, FeatureID("github.com/iota-uz/iota-sdk/pkg/clienthost/solid", "./Screen.tsx"), component.FeatureID())
	require.Equal(t, "./Screen.tsx", component.SourcePath())
	require.Equal(t, []Method{
		{Name: "example.load", Kind: MethodQuery, Cacheable: true, MaxRetries: 3},
		{Name: "example.save", Kind: MethodAction, Invalidates: []string{"example.load"}},
	}, component.Methods())
	rendered := component.Render(props{Name: "Granite"})
	require.Equal(t, component.FeatureID(), rendered.FeatureID())
	require.Equal(t, props{Name: "Granite"}, rendered.Props())
}

func TestImportRejectsInvalidRPCDeclarations(t *testing.T) {
	t.Parallel()
	tests := map[string]func(){
		"duplicate": func() {
			Import[props]("./Screen.tsx", WithRPC(Query[props, props]("same"), Action[props, props]("same")))
		},
		"action retry":         func() { Import[props]("./Screen.tsx", WithRPC(Action[props, props]("save", Cacheable(1)))) },
		"too many retries":     func() { Import[props]("./Screen.tsx", WithRPC(Query[props, props]("load", Cacheable(4)))) },
		"unknown invalidation": func() { Import[props]("./Screen.tsx", WithRPC(Action[props, props]("save", Invalidates("missing")))) },
	}
	for name, run := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Panics(t, run)
		})
	}
}

func TestImportRejectsUnsafeSourcePaths(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"Screen.tsx", "./Screen.ts", "../Screen.tsx", " ./Screen.tsx"} {
		t.Run(source, func(t *testing.T) {
			require.Panics(t, func() { Import[props](source) })
		})
	}
}
