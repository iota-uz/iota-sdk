package devrunner

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// packageJSONEngines is a minimal struct for reading engines from package.json.
type packageJSONEngines struct {
	Node string `json:"node"`
}

type packageJSON struct {
	Engines *packageJSONEngines `json:"engines"`
}

// PreflightFromPackageJSON parses package.json under projectRoot and returns the minimum Node major
// from engines.node (e.g. ">=18" -> 18, "18" -> 18). If file or field is missing, returns 0, nil.
func PreflightFromPackageJSON(projectRoot string) (minNodeMajor int, err error) {
	path := filepath.Join(projectRoot, "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return 0, nil // ignore malformed
	}
	if pkg.Engines == nil || pkg.Engines.Node == "" {
		return 0, nil
	}
	// Parse ">=18", "18", ">=18.0.0", "18.x", etc. — take first number as min major.
	re := regexp.MustCompile(`(\d+)`)
	m := re.FindStringSubmatch(pkg.Engines.Node)
	if len(m) < 2 {
		return 0, nil
	}
	major, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, nil
	}
	return major, nil
}

// PreflightNode checks that the current Node.js version meets the required major version (e.g. 18).
// Returns an error with remediation if the version is too old or node is not found.
func PreflightNode(requiredMajor int) error {
	out, err := exec.Command("node", "-v").Output()
	if err != nil {
		return fmt.Errorf("node not found or not runnable: %w\nremediation: install Node.js %d+ from https://nodejs.org/ or use nvm", err, requiredMajor)
	}
	v := strings.TrimSpace(string(out))
	// v is like "v20.10.0" or "v18.19.0"
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) == 0 {
		return fmt.Errorf("could not parse node version %q", string(out))
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("could not parse node major version %q: %w", parts[0], err)
	}
	if major < requiredMajor {
		return fmt.Errorf("node version %s is below required %d+\nremediation: upgrade Node.js (e.g. nvm install 20 or install from https://nodejs.org/)", v, requiredMajor)
	}
	return nil
}

// PreflightPnpm checks that pnpm is available and optionally prints its version.
// Returns an error if pnpm is not found.
func PreflightPnpm() error {
	out, err := exec.Command("pnpm", "-v").Output()
	if err != nil {
		return fmt.Errorf("pnpm not found: %w\nremediation: npm install -g pnpm or enable corepack", err)
	}
	_ = out // caller can log it if desired
	return nil
}

// pnpmListDep is one entry in the dependencies object from pnpm list --json.
type pnpmListDep struct {
	Version string `json:"version"`
}

// pnpmListEntry is one item in the top-level array from pnpm list --json.
type pnpmListEntry struct {
	Dependencies map[string]pnpmListDep `json:"dependencies"`
}

// PreflightDeps checks that react and react-dom resolve to a single version in the project
// (avoids "invalid hook call" from duplicate React instances). Uses pnpm list --json for stable parsing.
func PreflightDeps(projectRoot string) error {
	cmd := exec.Command("pnpm", "list", "react", "react-dom", "--depth", "0", "--json")
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		// pnpm list can exit non-zero if deps not installed; treat as warning, not hard fail
		return nil
	}
	var entries []pnpmListEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil // ignore malformed JSON; avoid breaking on pnpm output changes
	}
	reactVersions := make(map[string]bool)
	reactDomVersions := make(map[string]bool)
	for _, e := range entries {
		for name, dep := range e.Dependencies {
			if dep.Version == "" {
				continue
			}
			switch name {
			case "react":
				reactVersions[dep.Version] = true
			case "react-dom":
				reactDomVersions[dep.Version] = true
			}
		}
	}
	if len(reactVersions) > 1 || len(reactDomVersions) > 1 {
		return fmt.Errorf("multiple versions of react or react-dom detected (pnpm list in %s)\nremediation: dedupe with pnpm dedupe or align versions in package.json", projectRoot)
	}
	return nil
}
