package configurator_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/configurator"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func host() clienthost.Manifest {
	return clienthost.Manifest{
		Package: "@iota-uz/sdk", PackageVersion: "0.5.6",
		SDKCommit: "0123456789abcdef0123456789abcdef01234567", SDKReleaseVersion: sdkidentity.ReleaseVersion,
		ProtocolVersion: clienthost.ProtocolVersion, Entry: "/assets/host.js",
	}
}

func sdkTypes() fstest.MapFS {
	return fstest.MapFS{
		"@iota-uz/sdk/package.json":                     {Data: []byte(`{"name":"@iota-uz/sdk"}`)},
		"@iota-uz/sdk/dist/runtime/index.d.ts":          {Data: []byte("export declare const t: 1")},
		"@iota-uz/sdk/dist/runtime/index.js":            {Data: []byte("export const t = 1")},
		"@iota-uz/sdk/node_modules/left-pad/index.d.ts": {Data: []byte("nested")},
		"solid-js/types/index.d.ts":                     {Data: []byte("export declare function createSignal(): void")},
		"left-pad/index.d.ts":                           {Data: []byte("not an allowed package")},
	}
}

func newSuite(t *testing.T, perms ...permission.Permission) *itf.Suite {
	t.Helper()
	ctrl, err := configurator.NewController(configurator.Config{Host: host(), Service: &manage.Service{}, Types: sdkTypes()})
	require.NoError(t, err)
	suite := itf.NewSuite(t, modules.Components()...)
	suite.AsUser(itf.User(perms...)).Register(ctrl)
	return suite
}

func scriptJSON(t *testing.T, body, id string, into any) {
	t.Helper()
	marker := `id="` + id + `" type="application/json">`
	i := strings.Index(body, marker)
	require.GreaterOrEqual(t, i, 0, "script %s not found in page", id)
	rest := body[i+len(marker):]
	j := strings.Index(rest, "</script>")
	require.GreaterOrEqual(t, j, 0)
	require.NoError(t, json.Unmarshal([]byte(rest[:j]), into), rest[:j])
}

func TestPageIsAStandardClientRouteForReaders(t *testing.T) {
	suite := newSuite(t, corePerms.ExtensionsRead, corePerms.ExtensionsAuthor)
	resp := suite.GET("/extensions").Expect(t).Status(200)
	body := resp.Body()
	require.Contains(t, body, `id="iota-client-route"`, "mounts through the canonical client host element")
	require.Contains(t, body, `src="/assets/host.js"`)

	var rc struct {
		Initial     configurator.Props `json:"initial"`
		Route       struct{ ID, Path, FeatureID string }
		Permissions []string
		Locale      struct {
			Language string
			Messages map[string]string
		}
		Services struct{ RPC string }
		Theme    string
		CSRF     string
	}
	scriptJSON(t, body, "iota-client-context", &rc)
	assert.Equal(t, configurator.Props{BasePath: "/extensions", APIBase: "/_ext/manage", TypesBase: "/extensions/types"}, rc.Initial)
	assert.Equal(t, configurator.RouteID, rc.Route.ID)
	assert.Equal(t, "/extensions", rc.Route.Path)
	assert.Regexp(t, `^solid-[0-9a-f]{24}$`, rc.Route.FeatureID, "feature identity comes from solid.Import")
	assert.Contains(t, rc.Permissions, manage.PermAuthor)
	assert.Contains(t, rc.Locale.Messages, "Extensions.Configurator.Title")
	assert.Contains(t, rc.Locale.Messages, "Extensions.Configurator.ConflictText")
	assert.NotEmpty(t, rc.Locale.Language)
	assert.Contains(t, []string{"light", "dark"}, rc.Theme)
	assert.Empty(t, rc.CSRF, "no token is sent that nothing verifies; state-changing calls are guarded by the origin check of /_ext/manage")
}

func TestPageNeedsExtensionsRead(t *testing.T) {
	// whatever the navigation shows, the route refuses users without the permission
	suite := newSuite(t, corePerms.ExtensionsAuthor)
	suite.GET("/extensions").Expect(t).Status(403)
}

func TestTypeRoutesNeedExtensionsRead(t *testing.T) {
	suite := newSuite(t, corePerms.ExtensionsAuthor)
	suite.GET("/extensions/types/sdk").Expect(t).Status(403)
	suite.GET("/extensions/types/kit").Expect(t).Status(403)
}

func TestSDKTypesServeOnlyTheConfiguredTypingPackages(t *testing.T) {
	suite := newSuite(t, corePerms.ExtensionsRead)
	resp := suite.GET("/extensions/types/sdk").Expect(t).Status(200)
	assert.Equal(t, "application/json", resp.Header("Content-Type"))
	var out configurator.TypeFiles
	require.NoError(t, json.Unmarshal([]byte(resp.Body()), &out))
	paths := make([]string, 0, len(out.Files))
	for p := range out.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	assert.Equal(t, []string{"@iota-uz/sdk/dist/runtime/index.d.ts", "@iota-uz/sdk/package.json", "solid-js/types/index.d.ts"}, paths)
	assert.NotEmpty(t, out.Hash)
}

func TestSDKTypesAreEmptyWithoutAFilesystem(t *testing.T) {
	ctrl, err := configurator.NewController(configurator.Config{Host: host(), Service: &manage.Service{}})
	require.NoError(t, err)
	suite := itf.NewSuite(t, modules.Components()...)
	suite.AsUser(itf.User(corePerms.ExtensionsRead)).Register(ctrl)
	var out configurator.TypeFiles
	require.NoError(t, json.Unmarshal([]byte(suite.GET("/extensions/types/sdk").Expect(t).Status(200).Body()), &out))
	assert.Empty(t, out.Files)
}

func TestDescriptorPutsTheConfiguratorUnderAdministrationForReaders(t *testing.T) {
	ctrl, err := configurator.NewController(configurator.Config{Host: host(), Service: &manage.Service{}})
	require.NoError(t, err)
	d := ctrl.Descriptor()
	require.Len(t, d.Nav, 1)
	assert.Equal(t, "core.administration", d.Nav[0].Parent)
	assert.Equal(t, "/extensions", d.Nav[0].Path)
	assert.Equal(t, "NavigationLinks.Extensions", d.Nav[0].TitleKey)
	var page *application.RouteSpec
	for i := range d.Routes {
		if d.Routes[i].Path == "/extensions" {
			page = &d.Routes[i]
		}
	}
	require.NotNil(t, page)
	assert.Equal(t, application.RouteRendererClient, page.Renderer)
	assert.Equal(t, application.AuthPolicy{Permissions: []permission.Permission{corePerms.ExtensionsRead}, Logic: application.PermissionLogicAll}, page.Requirement)
	for _, r := range d.Routes {
		assert.False(t, r.Requirement.Public, r.Path)
		assert.NotEmpty(t, r.Requirement.Permissions, r.Path+" is guarded by Extensions.Read")
	}
}

func TestConfigRequiresAServiceAndAHost(t *testing.T) {
	_, err := configurator.NewController(configurator.Config{Host: host()})
	require.Error(t, err)
	_, err = configurator.NewController(configurator.Config{Service: &manage.Service{}})
	require.Error(t, err)
}

func TestKitFilesAreVirtualNodeModules(t *testing.T) {
	out := configurator.KitFiles(build.Kit{Hash: "k1", Modules: map[string]build.Tree{
		"@iota-kit/core": {"index.d.ts": []byte("export declare const a: 1"), "index.js": []byte("export const a = 1")},
		"@iota-kit/acme": {"index.d.ts": []byte("x"), "package.json": []byte(`{"name":"@iota-kit/acme"}`), "README.md": []byte("no")},
	}})
	assert.Equal(t, "k1", out.Hash)
	assert.Equal(t, "export declare const a: 1", out.Files["@iota-kit/core/index.d.ts"])
	assert.Contains(t, out.Files["@iota-kit/core/package.json"], `"types":"index.d.ts"`, "a module without package.json resolves through a generated one")
	assert.JSONEq(t, `{"name":"@iota-kit/acme"}`, out.Files["@iota-kit/acme/package.json"])
	assert.NotContains(t, out.Files, "@iota-kit/acme/README.md")
}

func TestMessagesFallBackToTheKey(t *testing.T) {
	msgs := configurator.Messages(context.Background())
	require.Len(t, msgs, len(configurator.MessageKeys))
	assert.Equal(t, "Title", msgs["Extensions.Configurator.Title"], "without a localizer the key is shown")
}

// Every string the screen asks for exists in every locale of the core module,
// and the screen asks for nothing that Go does not send.
func TestEveryScreenStringIsTranslatedInEveryLocale(t *testing.T) {
	files, err := filepath.Glob("modules/core/presentation/locales/*.json")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(files), 6)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		var doc map[string]any
		require.NoError(t, json.Unmarshal(raw, &doc), f)
		ext, _ := doc["Extensions"].(map[string]any)
		cfg, _ := ext["Configurator"].(map[string]any)
		nav, _ := doc["NavigationLinks"].(map[string]any)
		assert.NotEmpty(t, nav["Extensions"], f+": NavigationLinks.Extensions")
		for _, k := range configurator.MessageKeys {
			v, _ := cfg[k].(string)
			assert.NotEmpty(t, v, f+": Extensions.Configurator."+k)
		}
		assert.Len(t, cfg, len(configurator.MessageKeys), f+": no unused keys")
	}
}

func TestMessageKeysAreExactlyTheKeysTheScreenUses(t *testing.T) {
	used := map[string]bool{}
	re := regexp.MustCompile(`\bt\('([A-Za-z0-9]+)'\)`)
	files, err := filepath.Glob("pkg/extruntime/configurator/web/*.tsx")
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
			used[m[1]] = true
		}
	}
	got := make([]string, 0, len(used))
	for k := range used {
		got = append(got, k)
	}
	want := append([]string(nil), configurator.MessageKeys...)
	sort.Strings(got)
	sort.Strings(want)
	assert.Equal(t, want, got)
}

// countingFS counts the opens of the typings tree.
type countingFS struct {
	fs.FS // only Open is promoted: Stat, ReadFile and ReadDir go through it
	opens *atomic.Int64
}

func (c countingFS) Open(name string) (fs.File, error) {
	c.opens.Add(1)
	return c.FS.Open(name)
}

// The typings of the SDK come from the installed node_modules and cannot
// change under a running process: they are collected once and revalidated
// with an ETag, instead of walked and re-encoded for every editor session.
func TestSDKTypesAreComputedOnceAndRevalidated(t *testing.T) {
	var opens atomic.Int64
	ctrl, err := configurator.NewController(configurator.Config{Host: host(), Service: &manage.Service{}, Types: countingFS{sdkTypes(), &opens}})
	require.NoError(t, err)
	suite := itf.NewSuite(t, modules.Components()...)
	suite.AsUser(itf.User(corePerms.ExtensionsRead)).Register(ctrl)

	first := suite.GET("/extensions/types/sdk").Expect(t).Status(200)
	etag := first.Header("ETag")
	require.NotEmpty(t, etag)
	after := opens.Load()
	require.Positive(t, after)

	again := suite.GET("/extensions/types/sdk").Expect(t).Status(200)
	assert.Equal(t, first.Body(), again.Body())
	assert.Equal(t, etag, again.Header("ETag"))
	assert.Equal(t, after, opens.Load(), "the second request did not touch the tree")

	cached := suite.GET("/extensions/types/sdk").Header("If-None-Match", etag).Expect(t).Status(304)
	assert.Empty(t, cached.Body())
	assert.Equal(t, after, opens.Load())

	var out configurator.TypeFiles
	require.NoError(t, json.Unmarshal([]byte(first.Body()), &out))
	assert.Equal(t, `"`+out.Hash+`"`, etag, "the ETag is the digest of the typings")
}
