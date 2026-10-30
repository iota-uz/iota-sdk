package logging_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

const coverageMatrix = "modules/logging/BUSINESS_HISTORY.md"

// coveredModules are the modules whose application services must account for every mutation.
var coveredModules = []string{"core", "crm", "finance", "hrm", "projects", "warehouse"}

var (
	mutationVerb  = regexp.MustCompile(`^(Create|Update|Delete|Save|Upsert|Add|Remove|Set|Attach|Detach|Accept|Reject|Complete|Cancel|Close|Approve|Archive|Restore|Transfer|Settle|WriteOff|Assign|Unassign|Import|Bulk|Mark|Move|Change|Reverse|Post|Block|Unblock|Enable|Disable|Grant|Revoke|Load|Register|Reset|Invite|Activate|Deactivate|Apply|Record|Recalculate|Sync|Upload|Link|Unlink|Pay|Send|Revert|Sign|Unsign|Submit|Confirm|Reopen|Write|Issue)`)
	matrixMethod  = regexp.MustCompile(`\b([a-z]+/[A-Z][A-Za-z0-9]*Service)\.([A-Z][A-Za-z0-9]*)\b`)
	matrixEvent   = regexp.MustCompile("(?m)^\\|\\s*`([a-z][a-z0-9_]*(?:\\.[a-z][a-z0-9_]*)+)`\\s*\\|")
	serviceSuffix = "Service"
)

// serviceMutations lists every exported mutating method of an application service type in the
// covered modules, as module/Service.Method.
func serviceMutations(t *testing.T) []string {
	t.Helper()
	var result []string
	for _, module := range coveredModules {
		root := filepath.Join("modules", module, "services")
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv == nil || !fn.Name.IsExported() || !mutationVerb.MatchString(fn.Name.Name) {
					continue
				}
				recv := receiverName(fn.Recv.List[0].Type)
				if !strings.HasSuffix(recv, serviceSuffix) || !ast.IsExported(recv) {
					continue
				}
				result = append(result, module+"/"+recv+"."+fn.Name.Name)
			}
			return nil
		})
		require.NoError(t, err)
	}
	sort.Strings(result)
	return result
}

func receiverName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return receiverName(e.X)
	case *ast.Ident:
		return e.Name
	case *ast.IndexExpr:
		return receiverName(e.X)
	}
	return ""
}

func TestBusinessHistoryMatrix_CoversEveryServiceMutation(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(coverageMatrix)
	require.NoError(t, err)
	listed := map[string]bool{}
	for _, m := range matrixMethod.FindAllStringSubmatch(string(data), -1) {
		listed[m[1]+"."+m[2]] = true
	}
	var missing []string
	for _, method := range serviceMutations(t) {
		if !listed[method] {
			missing = append(missing, method)
		}
	}
	require.Empty(t, missing, "service mutations without a business event or an explicit exception in %s", coverageMatrix)
}

func TestBusinessHistoryMatrix_MatchesRegistry(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(coverageMatrix)
	require.NoError(t, err)
	documented := map[string]bool{}
	for _, m := range matrixEvent.FindAllStringSubmatch(string(data), -1) {
		documented[m[1]] = true
	}
	env := itf.Setup(t, itf.WithComponents(modules.Components()...))
	registry := resolve[*history.Registry](t, env)
	registered := map[string]bool{}
	var undocumented []string
	for _, d := range registry.Definitions() {
		registered[d.Type] = true
		if !documented[d.Type] && d.Type != history.LegacyActionLogType {
			undocumented = append(undocumented, d.Type)
		}
	}
	var unregistered []string
	for eventType := range documented {
		if !registered[eventType] {
			unregistered = append(unregistered, eventType)
		}
	}
	require.Empty(t, undocumented, "registered event types missing from %s", coverageMatrix)
	require.Empty(t, unregistered, "event types in %s that no component registers", coverageMatrix)
}

func TestBusinessHistoryDefinitions_TranslatedInEveryLocale(t *testing.T) {
	t.Parallel()
	env := itf.Setup(t, itf.WithComponents(modules.Components()...))
	messages := env.App.Bundle().Messages()
	require.NotEmpty(t, messages)
	var missing []string
	for _, d := range resolve[*history.Registry](t, env).Definitions() {
		keys := []string{d.TitleKey, d.DescriptionKey}
		for _, r := range d.Resources {
			for _, f := range r.Fields {
				keys = append(keys, f.LabelKey)
			}
		}
		for tag, byID := range messages {
			for _, key := range keys {
				if _, ok := byID[key]; !ok {
					missing = append(missing, tag.String()+" "+d.Type+" "+key)
				}
			}
		}
	}
	sort.Strings(missing)
	require.Empty(t, missing, "business history labels without a translation")
}
