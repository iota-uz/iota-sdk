package controllers_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/stretchr/testify/require"
)

func flattenLocale(prefix string, node map[string]any, out map[string]struct{}) {
	for key, value := range node {
		if nested, ok := value.(map[string]any); ok {
			flattenLocale(prefix+key+".", nested, out)
			continue
		}
		out[prefix+key] = struct{}{}
	}
}

func loadLocaleKeys(t *testing.T, locale string) map[string]struct{} {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	modulesDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	keys := make(map[string]struct{})
	for _, ext := range []string{"json", "toml"} {
		files, err := filepath.Glob(filepath.Join(modulesDir, "*", "presentation", "locales", locale+"."+ext))
		require.NoError(t, err)
		for _, file := range files {
			payload, err := os.ReadFile(file)
			require.NoError(t, err)
			var tree map[string]any
			if ext == "json" {
				require.NoError(t, json.Unmarshal(payload, &tree))
			} else {
				require.NoError(t, toml.Unmarshal(payload, &tree))
			}
			flattenLocale("", tree, keys)
		}
	}
	require.NotEmpty(t, keys)
	return keys
}

func TestDefaultPermissionSchemaIsTranslated(t *testing.T) {
	t.Parallel()

	groups := controllers.BuildModulePermissionGroups(defaults.PermissionSchema())
	require.NotEmpty(t, groups)

	for _, locale := range []string{"en", "ru", "uz", "uz-Cyrl", "zh", "pt-BR"} {
		t.Run(locale, func(t *testing.T) {
			t.Parallel()
			keys := loadLocaleKeys(t, locale)
			var missing []string
			need := func(key string) {
				if _, ok := keys[key]; !ok {
					missing = append(missing, key)
				}
			}
			for _, module := range groups {
				need("Modules." + module.Module)
				for _, resource := range module.ResourceGroups {
					need("Resources." + resource.Resource)
					for _, set := range resource.PermissionSets {
						need(set.Label)
						if set.Description != "" {
							need(set.Description)
						}
						for _, perm := range set.Permissions {
							need("Permissions." + perm.Name)
						}
					}
				}
			}
			require.Empty(t, missing, "missing %s translations:\n%s", locale, strings.Join(missing, "\n"))
		})
	}
}
