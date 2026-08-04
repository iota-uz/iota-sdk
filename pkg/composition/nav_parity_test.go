package composition_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/bichat"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

var updateNavParityGolden = flag.Bool("update-nav-parity", false, "update SDK navigation parity golden")

type navParitySnapshot struct {
	NavItems   []navParityItem      `json:"nav_items"`
	QuickLinks []navParityQuickLink `json:"quick_links"`
}

type navParityItem struct {
	Key         string          `json:"key"`
	Name        string          `json:"name"`
	Href        string          `json:"href,omitempty"`
	Workspace   string          `json:"workspace,omitempty"`
	Keywords    []string        `json:"keywords,omitempty"`
	Order       int             `json:"order"`
	Children    []navParityItem `json:"children,omitempty"`
	Permissions []string        `json:"permissions,omitempty"`
	Logic       string          `json:"logic,omitempty"`
	IsBeta      bool            `json:"is_beta,omitempty"`
}

type navParityQuickLink struct {
	TrKey       string                 `json:"tr_key"`
	Link        string                 `json:"link"`
	Access      spotlight.AccessPolicy `json:"access"`
	Permissions []string               `json:"permissions,omitempty"`
}

func TestSDKNavigationParityGolden(t *testing.T) {
	snapshot := buildSDKNavigationParitySnapshot(t)
	got, err := json.MarshalIndent(snapshot, "", "  ")
	require.NoError(t, err)
	got = append(got, '\n')

	goldenPath := filepath.Join("testdata", "sdk_nav_parity.golden.json")
	if *updateNavParityGolden {
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o755))
		require.NoError(t, os.WriteFile(goldenPath, got, 0o644))
	}

	want, err := os.ReadFile(goldenPath)
	require.NoError(t, err)

	var expected navParitySnapshot
	require.NoError(t, json.Unmarshal(want, &expected))

	var actual navParitySnapshot
	require.NoError(t, json.Unmarshal(got, &actual))

	expectedNav, actualNav := normalizeNavForStructure(
		stripNavPermissions(expected.NavItems),
		stripNavPermissions(actual.NavItems),
	)
	require.Equal(t, expectedNav, actualNav)
	expectedQuickLinks := filterGroupQuickLinks(expected.QuickLinks, expected.NavItems)
	actualQuickLinks := filterGroupQuickLinks(actual.QuickLinks, expected.NavItems)
	requireQuickLinkStructureEqual(t, expectedQuickLinks, actualQuickLinks)
	requireNavPermissionsSuperset(t, expected.NavItems, actual.NavItems)
	requireQuickLinkAccessSuperset(t, expectedQuickLinks, actualQuickLinks)
}

func buildSDKNavigationParitySnapshot(t *testing.T) navParitySnapshot {
	t.Helper()

	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	poolConfig, err := pgxpool.ParseConfig("postgres://iota:iota@127.0.0.1:1/iota?sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	src, err := config.Build(static.New(nil))
	require.NoError(t, err)

	app, err := application.New(&application.ApplicationOptions{
		Pool:               pool,
		Bundle:             application.LoadBundle(),
		EventBus:           eventbus.NewEventPublisher(logger),
		Logger:             logger,
		SupportedLanguages: application.DefaultSupportedLanguages(),
	})
	require.NoError(t, err)

	engine := composition.NewEngine()
	components := append(modules.Components(), bichat.NewComponent())
	require.NoError(t, engine.Register(components...))

	container, err := engine.Compile(
		composition.NewBuildContext(app, src, composition.WithLogger(logger)),
		composition.CapabilityAPI,
	)
	require.NoError(t, err)

	return navParitySnapshot{
		NavItems:   snapshotNavItems(container.NavItems()),
		QuickLinks: snapshotQuickLinkProvider(t, app.QuickLinks()),
	}
}

func snapshotNavItems(items []types.NavigationItem) []navParityItem {
	out := make([]navParityItem, 0, len(items))
	for index, item := range items {
		out = append(out, navParityItem{
			Key:         item.Key,
			Name:        item.Name,
			Href:        item.Href,
			Workspace:   item.Workspace,
			Keywords:    append([]string(nil), item.Keywords...),
			Order:       index,
			Children:    snapshotNavItems(item.Children),
			Permissions: permissionNames(item.Permissions),
			Logic:       navPermissionLogic(item.Logic),
			IsBeta:      item.IsBeta,
		})
	}
	return out
}

func snapshotQuickLinkProvider(t *testing.T, quickLinks *spotlight.QuickLinks) []navParityQuickLink {
	t.Helper()

	docs, err := spotlight.CollectDocuments(context.Background(), quickLinks, spotlight.ProviderScope{
		TenantID: uuid.New(),
		Language: "en",
	})
	require.NoError(t, err)

	out := make([]navParityQuickLink, 0, len(docs))
	for _, doc := range docs {
		trKey := doc.Metadata["tr_key"]
		permissions := append([]string(nil), doc.Access.AllowedPermissions...)
		sort.Strings(permissions)
		doc.Access.AllowedPermissions = permissions
		out = append(out, navParityQuickLink{
			TrKey:       trKey,
			Link:        doc.URL,
			Access:      doc.Access,
			Permissions: permissions,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TrKey != out[j].TrKey {
			return out[i].TrKey < out[j].TrKey
		}
		return out[i].Link < out[j].Link
	})
	return out
}

func permissionNames(perms []permission.Permission) []string {
	names := make([]string, 0, len(perms))
	for _, perm := range perms {
		if perm == nil || perm.Name() == "" {
			continue
		}
		names = append(names, perm.Name())
	}
	sort.Strings(names)
	return names
}

func navPermissionLogic(logic types.PermissionLogic) string {
	if logic == types.PermissionLogicAny {
		return "any"
	}
	return "all"
}

func stripNavPermissions(items []navParityItem) []navParityItem {
	out := make([]navParityItem, len(items))
	for i, item := range items {
		item.Permissions = nil
		item.Logic = ""
		item.Children = stripNavPermissions(item.Children)
		out[i] = item
	}
	return out
}

func normalizeNavForStructure(expected, actual []navParityItem) ([]navParityItem, []navParityItem) {
	actual = alignNavSiblings(expected, actual)
	want := make([]navParityItem, len(expected))
	got := make([]navParityItem, len(actual))
	limit := len(expected)
	if len(actual) < limit {
		limit = len(actual)
	}
	for i := 0; i < limit; i++ {
		want[i] = expected[i]
		got[i] = actual[i]
		if want[i].Key == "" {
			got[i].Key = ""
		}
		if len(want[i].Children) > 0 && got[i].Href == "" {
			want[i].Href = ""
		}
		got[i].Order = want[i].Order
		want[i].Children, got[i].Children = normalizeNavForStructure(want[i].Children, got[i].Children)
	}
	for i := limit; i < len(expected); i++ {
		want[i] = expected[i]
	}
	for i := limit; i < len(actual); i++ {
		got[i] = actual[i]
	}
	return want, got
}

func alignNavSiblings(expected, actual []navParityItem) []navParityItem {
	aligned := make([]navParityItem, 0, len(actual))
	used := make([]bool, len(actual))
	for _, want := range expected {
		for i, got := range actual {
			if used[i] || !sameNavShape(want, got) {
				continue
			}
			aligned = append(aligned, got)
			used[i] = true
			break
		}
	}
	for i, got := range actual {
		if !used[i] {
			aligned = append(aligned, got)
		}
	}
	return aligned
}

func sameNavShape(expected, actual navParityItem) bool {
	if expected.Name != actual.Name {
		return false
	}
	if expected.Key != "" && expected.Key != actual.Key {
		return false
	}
	if len(expected.Children) > 0 && actual.Href == "" {
		return true
	}
	return expected.Href == actual.Href
}

func requireQuickLinkStructureEqual(t *testing.T, expected, actual []navParityQuickLink) {
	t.Helper()
	require.Len(t, actual, len(expected))
	for i := range expected {
		require.Equal(t, expected[i].TrKey, actual[i].TrKey)
		require.Equal(t, expected[i].Link, actual[i].Link)
	}
}

func filterGroupQuickLinks(links []navParityQuickLink, items []navParityItem) []navParityQuickLink {
	groupLinks := make(map[string]struct{})
	var collect func([]navParityItem)
	collect = func(items []navParityItem) {
		for _, item := range items {
			if item.Href != "" && len(item.Children) > 0 {
				groupLinks[item.Name+" "+item.Href] = struct{}{}
			}
			collect(item.Children)
		}
	}
	collect(items)

	out := make([]navParityQuickLink, 0, len(links))
	for _, link := range links {
		if _, ok := groupLinks[link.TrKey+" "+link.Link]; ok {
			continue
		}
		out = append(out, link)
	}
	return out
}

func requireNavPermissionsSuperset(t *testing.T, expected, actual []navParityItem) {
	t.Helper()
	actual = alignNavSiblings(expected, actual)
	require.Len(t, actual, len(expected))
	for i := range expected {
		requirePermissionsSuperset(t, expected[i].Permissions, actual[i].Permissions, expected[i].Key)
		if len(expected[i].Permissions) > 0 {
			require.Equal(t, expected[i].Logic, actual[i].Logic, "permission logic changed for %s", expected[i].Key)
		}
		requireNavPermissionsSuperset(t, expected[i].Children, actual[i].Children)
	}
}

func requireQuickLinkAccessSuperset(t *testing.T, expected, actual []navParityQuickLink) {
	t.Helper()
	for i := range expected {
		label := expected[i].TrKey + " " + expected[i].Link
		requirePermissionsSuperset(t, expected[i].Permissions, actual[i].Permissions, label)
		if len(expected[i].Permissions) > 0 {
			require.Equal(t, expected[i].Access.PermissionLogic, actual[i].Access.PermissionLogic, "permission logic changed for %s", label)
		}
		if len(actual[i].Permissions) == 0 {
			require.Equal(t, expected[i].Access.Visibility, actual[i].Access.Visibility, "visibility weakened for %s", label)
		}
	}
}

func requirePermissionsSuperset(t *testing.T, expected, actual []string, label string) {
	t.Helper()
	actualSet := make(map[string]struct{}, len(actual))
	for _, perm := range actual {
		actualSet[perm] = struct{}{}
	}
	for _, perm := range expected {
		_, ok := actualSet[perm]
		require.Truef(t, ok, "%s lost permission %s", label, perm)
	}
}
