package document_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/lens/document"
)

var (
	literalKeyPattern = regexp.MustCompile(`translate\(\s*'([^']+)'`)
	dynamicKeyPattern = regexp.MustCompile("translate\\(\\s*`([^`$]*)\\$\\{")
	// Keys resolved indirectly through catalog definitions, e.g. the period
	// preset catalog's `labelKey: 'filter.period.preset.today'` entries that
	// reach translate() as `translate(def.labelKey, def.fallback)`.
	catalogKeyPattern = regexp.MustCompile(`labelKey:\s*'([^']+)'`)
)

// TestRuntimeI18nKeysMatchRuntimeCallSites keeps the Go-side catalogue and the
// TSX call sites from drifting apart in either direction.
func TestRuntimeI18nKeysMatchRuntimeCallSites(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "..", "web", "lens", "src")
	if _, err := os.Stat(root); err != nil {
		t.Skipf("runtime sources are unavailable: %v", err)
	}

	found := make(map[string]struct{})
	prefixes := make(map[string]struct{})
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.Contains(path, ".test.") {
			return nil
		}
		if extension := filepath.Ext(path); extension != ".ts" && extension != ".tsx" {
			return nil
		}
		source, readErr := os.ReadFile(path) //nolint:gosec // test-only walk of repository sources
		if readErr != nil {
			return readErr
		}
		for _, match := range literalKeyPattern.FindAllStringSubmatch(string(source), -1) {
			found[match[1]] = struct{}{}
		}
		for _, match := range catalogKeyPattern.FindAllStringSubmatch(string(source), -1) {
			found[match[1]] = struct{}{}
		}
		for _, match := range dynamicKeyPattern.FindAllStringSubmatch(string(source), -1) {
			prefixes[match[1]] = struct{}{}
		}
		return nil
	})
	require.NoError(t, err)

	declared := make(map[string]struct{}, len(document.RuntimeI18nKeys()))
	for _, key := range document.RuntimeI18nKeys() {
		declared[key] = struct{}{}
	}

	missing := make([]string, 0)
	for key := range found {
		if _, ok := declared[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	require.Empty(t, missing, "runtime uses translation keys the Go catalogue does not declare")

	// Dynamic keys are declared per concrete suffix; assert the family exists.
	for prefix := range prefixes {
		require.Contains(t, prefixes, document.I18nSemanticsPrefix,
			"unexpected dynamic translation prefix %q", prefix)
	}
	for _, semantics := range []document.Semantics{
		document.SemanticsEvidence, document.SemanticsPartition,
		document.SemanticsReconciliation, document.SemanticsSeries,
	} {
		require.Contains(t, declared, document.I18nSemanticsPrefix+string(semantics))
	}

	unused := make([]string, 0)
	for key := range declared {
		if _, ok := found[key]; ok {
			continue
		}
		if strings.HasPrefix(key, document.I18nSemanticsPrefix) {
			continue
		}
		unused = append(unused, key)
	}
	sort.Strings(unused)
	require.Empty(t, unused, "Go catalogue declares translation keys no runtime call site uses")
}
