package integrations

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/automations/presentation/bundle"
	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
)

// AssetsPath is where the module serves its own bundle: <AssetsPath>/<release>/assets/<file>.
const AssetsPath = "/assets/automations"

// AssetsDirEnv points the module at a freshly built dist directory (development).
const AssetsDirEnv = "AUTOMATIONS_ASSETS_DIR"

// dist is the screen's bundle, built by web/scripts/build-bundle.mjs and committed, so an application
// that lists the module serves the page without merging the feature into its own client bundle.
//
//go:embed all:dist
var embedded embed.FS

// Bundle is the module's own client bundle: the host manifest that renders the screen and the handler
// that serves its files under AssetsPath.
type Bundle = bundle.Bundle

// DefaultBundle is the embedded bundle (or the one at AUTOMATIONS_ASSETS_DIR), for this binary's SDK
// release.
func DefaultBundle() (Bundle, error) {
	files, err := bundleFiles()
	if err != nil {
		return Bundle{}, err
	}
	b, err := bundleFor(files, sdkidentity.ReleaseVersion)
	if err != nil {
		return Bundle{}, err
	}
	if err := b.Manifest.Validate(); err != nil {
		return Bundle{}, fmt.Errorf("automations bundle: %w", err)
	}
	return b, nil
}

func bundleFiles() (fs.FS, error) {
	if dir := strings.TrimSpace(os.Getenv(AssetsDirEnv)); dir != "" {
		return os.DirFS(dir), nil
	}
	return fs.Sub(embedded, "dist")
}

// bundleFor is the bundle served as SDK release (bundle.Load).
func bundleFor(files fs.FS, release string) (Bundle, error) {
	return bundle.Load(files, AssetsPath, "@iota-uz/sdk/automations-integrations", release)
}
