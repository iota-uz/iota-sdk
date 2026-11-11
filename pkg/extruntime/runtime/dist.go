package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// DistEnv names the environment variable that points at the installed
// @iota-uz/sdk package directory (the one holding its package.json), for
// deployments whose node_modules is not under the application root.
const DistEnv = "IOTA_SDK_DIST"

// SDKPackage is the npm package that ships the Node workers.
const SDKPackage = "@iota-uz/sdk"

// Paths of the workers inside the @iota-uz/sdk package (web/runtime/dist is
// assembled into dist/runtime by web/sdk/scripts/assemble-subpaths.mjs).
const (
	distHostWorker  = "dist/runtime/host/worker.mjs"
	distBuildWorker = "dist/runtime/build/worker.mjs"
	distIdentity    = "dist/sdk-identity.json"
)

// DistConfig locates the installed @iota-uz/sdk distribution. The Go module
// and the npm package are one release unit (docs/client-host-release-train.md):
// the workers are taken from the npm package the application installs with
// the same version as its go.mod, never from the Go module cache.
type DistConfig struct {
	// Dir is the @iota-uz/sdk package directory. Default: $IOTA_SDK_DIST,
	// else <AppRoot>/node_modules/@iota-uz/sdk.
	Dir string
	// AppRoot is the application's JavaScript root (the directory holding
	// its package.json and node_modules). Default: the working directory.
	AppRoot string
}

// Dist is a verified @iota-uz/sdk distribution.
type Dist struct {
	// Dir is the package directory.
	Dir string
	// HostWorker is the invocation pool worker (Config.WorkerScript).
	HostWorker string
	// BuildWorker is the build worker (build.Config.WorkerScript).
	BuildWorker string
	// Identity is the distribution's release identity, equal in release and
	// protocol major to the Go SDK's (pkg/sdkidentity).
	Identity sdkidentity.Identity
}

// ResolveDist finds the @iota-uz/sdk distribution and verifies that it is the
// release of this Go SDK: the package name, its version and the release
// version of its identity must equal sdkidentity.ReleaseVersion, the protocol
// major must match, and both workers must be present. Every failure names the
// file it read and how to fix the installation.
func ResolveDist(cfg DistConfig) (Dist, error) {
	const op = serrors.Op("extruntime.runtime.ResolveDist")
	dir, origin, err := distDir(cfg)
	if err != nil {
		return Dist{}, serrors.Wrap(op, err)
	}
	fail := func(format string, args ...any) (Dist, error) {
		msg := fmt.Sprintf(format, args...)
		return Dist{}, serrors.New(serrors.FailedPrecondition, fmt.Sprintf(
			"extruntime: %s package at %s (%s): %s; install %s@%s next to the Go module of the same version, or set %s",
			SDKPackage, dir, origin, msg, SDKPackage, sdkidentity.ReleaseVersion, DistEnv)).WithOp(op)
	}
	var pkg struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := readJSON(filepath.Join(dir, "package.json"), &pkg); err != nil {
		return fail("%v", err)
	}
	if pkg.Name != SDKPackage {
		return fail("package.json names %q, not %s", pkg.Name, SDKPackage)
	}
	if pkg.Version != sdkidentity.ReleaseVersion {
		return fail("version %s does not match the Go SDK release %s", pkg.Version, sdkidentity.ReleaseVersion)
	}
	var id sdkidentity.Identity
	if err := readJSON(filepath.Join(dir, distIdentity), &id); err != nil {
		return fail("%v", err)
	}
	if err := id.Validate(); err != nil {
		return fail("%s: %v", distIdentity, err)
	}
	d := Dist{Dir: dir, HostWorker: filepath.Join(dir, distHostWorker), BuildWorker: filepath.Join(dir, distBuildWorker), Identity: id}
	for _, f := range []string{d.HostWorker, d.BuildWorker} {
		if _, err := os.Stat(f); err != nil {
			return fail("worker %s is missing (%v)", f, err)
		}
	}
	return d, nil
}

func distDir(cfg DistConfig) (string, string, error) {
	dir, origin := cfg.Dir, "DistConfig.Dir"
	if dir == "" {
		if env := os.Getenv(DistEnv); env != "" {
			dir, origin = env, "$"+DistEnv
		}
	}
	if dir == "" {
		root := cfg.AppRoot
		if root == "" {
			wd, err := os.Getwd()
			if err != nil {
				return "", "", err
			}
			root = wd
		}
		dir, origin = filepath.Join(root, "node_modules", "@iota-uz", "sdk"), "application node_modules"
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", err
	}
	return abs, origin, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s does not exist", path)
		}
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
