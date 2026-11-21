package integrations

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
)

// AssetsPath is where the module serves its own bundle.
const AssetsPath = "/assets/automations"

// AssetsDirEnv points the module at a freshly built dist directory (development).
const AssetsDirEnv = "AUTOMATIONS_ASSETS_DIR"

// dist is the screen's bundle, built by web/scripts/build-bundle.mjs and committed, so an application
// that lists the module serves the page without merging the feature into its own client bundle.
//
//go:embed all:dist
var embedded embed.FS

type bundleFile struct {
	SDKCommit         string   `json:"sdkCommit"`
	SDKReleaseVersion string   `json:"sdkReleaseVersion"`
	ProtocolVersion   string   `json:"protocolVersion"`
	Entry             string   `json:"entry"`
	Styles            []string `json:"styles"`
}

// Bundle is the module's own client bundle: the host manifest that renders the screen and the files to
// serve under AssetsPath.
type Bundle struct {
	Manifest clienthost.Manifest
	Files    fs.FS
}

// DefaultBundle is the embedded bundle (or the one at AUTOMATIONS_ASSETS_DIR). It is refused when it
// was built from another SDK release than this binary's.
func DefaultBundle() (Bundle, error) {
	var files fs.FS
	if dir := strings.TrimSpace(os.Getenv(AssetsDirEnv)); dir != "" {
		files = os.DirFS(dir)
	} else {
		sub, err := fs.Sub(embedded, "dist")
		if err != nil {
			return Bundle{}, err
		}
		files = sub
	}
	raw, err := fs.ReadFile(files, "bundle.json")
	if err != nil {
		return Bundle{}, fmt.Errorf("automations bundle: %w (build it with web/scripts/build-bundle.mjs)", err)
	}
	var b bundleFile
	if err := json.Unmarshal(raw, &b); err != nil {
		return Bundle{}, fmt.Errorf("automations bundle: bundle.json: %w", err)
	}
	if b.SDKReleaseVersion != sdkidentity.ReleaseVersion {
		return Bundle{}, fmt.Errorf("automations bundle: built for SDK %s, this binary is %s: rebuild it", b.SDKReleaseVersion, sdkidentity.ReleaseVersion)
	}
	m := clienthost.Manifest{
		Package: "@iota-uz/sdk/automations-integrations", PackageVersion: b.SDKReleaseVersion,
		SDKCommit: b.SDKCommit, SDKReleaseVersion: b.SDKReleaseVersion, ProtocolVersion: b.ProtocolVersion,
		Entry: path.Join(AssetsPath, b.Entry),
	}
	for _, s := range b.Styles {
		m.Styles = append(m.Styles, path.Join(AssetsPath, s))
	}
	if err := m.Validate(); err != nil {
		return Bundle{}, fmt.Errorf("automations bundle: %w", err)
	}
	return Bundle{Manifest: m, Files: files}, nil
}

// assetHandler serves the bundle's files; names carry content hashes, so they are cached for good.
func assetHandler(files fs.FS) http.Handler {
	server := http.StripPrefix(AssetsPath, http.FileServer(http.FS(files)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, AssetsPath+"/assets/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		server.ServeHTTP(w, r)
	})
}
