package rpccodegen

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateGoIdentifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "Valid", input: "Router", wantErr: false},
		{name: "ValidWithUnderscore", input: "_router2", wantErr: false},
		{name: "Empty", input: "", wantErr: true},
		{name: "InvalidDash", input: "router-func", wantErr: true},
		{name: "InvalidLeadingDigit", input: "1Router", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateGoIdentifier(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestInspectRouter(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRoot(t)
	modPath, err := ReadModulePath(filepath.Join(repoRoot, "go.mod"))
	require.NoError(t, err)
	importPath := modPath + "/internal/applet/rpccodegen/testfixtures/routerfixtures"

	t.Run("NoArgsRouter", func(t *testing.T) {
		t.Parallel()
		desc, err := InspectRouter(repoRoot, importPath, "Router")
		require.NoError(t, err)
		require.NotNil(t, desc)
		require.NotEmpty(t, desc.Methods)
		require.Equal(t, "fixtures.ping", desc.Methods[0].Name)
	})

	t.Run("DependencyfulRouter", func(t *testing.T) {
		t.Parallel()
		desc, err := InspectRouter(repoRoot, importPath, "RouterWithDeps")
		require.NoError(t, err)
		require.NotNil(t, desc)
		require.NotEmpty(t, desc.Methods)
		require.Equal(t, "fixtures.ping", desc.Methods[0].Name)
	})

	t.Run("InvalidReturnType", func(t *testing.T) {
		t.Parallel()
		_, err := InspectRouter(repoRoot, importPath, "RouterBadReturn")
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected *applet.TypedRPCRouter")
	})
}

func TestBuildRouterInspectorProgram(t *testing.T) {
	t.Parallel()

	code := BuildRouterInspectorProgram(
		"github.com/iota-uz/iota-sdk",
		"github.com/iota-uz/iota-sdk/modules/bichat/rpc",
		"Router",
	)
	require.Contains(t, code, `reflect.ValueOf(rpc.Router)`)
	require.Contains(t, code, `const routerFuncName = "Router"`)
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	wd, err := filepath.Abs(".")
	require.NoError(t, err)
	for {
		if _, readErr := ReadModulePath(filepath.Join(wd, "go.mod")); readErr == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			break
		}
		wd = parent
	}
	t.Fatal("repo root with go.mod not found")
	return ""
}
