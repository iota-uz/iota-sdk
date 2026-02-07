package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAppletName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{name: "ValidLowercase", input: "bichat", wantError: false},
		{name: "ValidWithDashAndUnderscore", input: "foo-bar_baz", wantError: false},
		{name: "Missing", input: "", wantError: true},
		{name: "StartsWithDigit", input: "1bichat", wantError: true},
		{name: "InvalidChars", input: "bi.chat", wantError: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateAppletName(tt.input)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestTypeNameFromAppletName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "Simple", input: "bichat", want: "BichatRPC"},
		{name: "Dash", input: "foo-bar", want: "FooBarRPC"},
		{name: "Underscore", input: "foo_bar", want: "FooBarRPC"},
		{name: "DashAndDigits", input: "foo-2bar", want: "Foo2barRPC"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, typeNameFromAppletName(tt.input))
		})
	}
}

func TestBuildRPCConfig_TargetSelection(t *testing.T) {
	t.Parallel()

	t.Run("PreferSDKPathWhenDataDirExists", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, "ui", "src", "bichat", "data"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(root, "modules", "bichat", "presentation", "web", "src"), 0o755))

		cfg, err := buildRPCConfig(root, "bichat", "Router")
		require.NoError(t, err)
		assert.Equal(t, "ui/src/bichat/data/rpc.generated.ts", cfg.TargetOut)
		assert.Equal(t, "BichatRPC", cfg.TypeName)
	})

	t.Run("FallbackToModulePath", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, "modules", "foo", "presentation", "web", "src"), 0o755))

		cfg, err := buildRPCConfig(root, "foo", "Router")
		require.NoError(t, err)
		assert.Equal(t, "modules/foo/presentation/web/src/rpc.generated.ts", cfg.TargetOut)
		assert.Equal(t, "modules/foo/rpc", cfg.RouterPackage)
	})
}

func TestBichatReexportContent(t *testing.T) {
	t.Parallel()
	expected := "// Re-export canonical RPC contract from @iota-uz/sdk package.\n" +
		"export type { BichatRPC } from '@iota-uz/sdk/bichat'\n"
	assert.Equal(t, expected, bichatReexportContent("BichatRPC"))
}

func TestSetEnv(t *testing.T) {
	t.Parallel()
	env := []string{"A=1", "GOTOOLCHAIN=local", "B=2"}
	out := setEnv(env, "GOTOOLCHAIN", "auto")
	assert.Contains(t, out, "A=1")
	assert.Contains(t, out, "B=2")
	assert.Contains(t, out, "GOTOOLCHAIN=auto")
	assert.NotContains(t, out, "GOTOOLCHAIN=local")
}
