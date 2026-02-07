package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/cmd/applet-rpc-typegen/internal/typegen"
	"github.com/iota-uz/iota-sdk/pkg/applet"
)

func main() {
	var routerPkg string
	var routerFunc string
	var outPath string
	var typeName string

	flag.StringVar(&routerPkg, "router-pkg", "", "Router package import path or module-relative path (e.g. modules/bichat/rpc)")
	flag.StringVar(&routerFunc, "router-func", "Router", "Router factory function name in router package (e.g. Router)")
	flag.StringVar(&outPath, "out", "", "Output TypeScript file path")
	flag.StringVar(&typeName, "type-name", "", "Root TypeScript RPC type name (e.g. BichatRPC)")
	flag.Parse()

	if strings.TrimSpace(routerPkg) == "" || strings.TrimSpace(routerFunc) == "" || strings.TrimSpace(outPath) == "" || strings.TrimSpace(typeName) == "" {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: applet-rpc-typegen --router-pkg <pkg> --router-func <func> --out <file> --type-name <TypeName>")
		os.Exit(2)
	}

	root, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	routerImport, err := resolveRouterImport(root, routerPkg)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	desc, err := inspectRouter(root, routerImport, routerFunc)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	ts, err := typegen.EmitTypeScript(desc, typeName)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, []byte(ts), 0o644); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func resolveRouterImport(repoRoot string, routerPkg string) (string, error) {
	routerPkg = strings.TrimSpace(routerPkg)
	if routerPkg == "" {
		return "", fmt.Errorf("router package is empty")
	}
	if strings.HasPrefix(routerPkg, "modules/") || strings.HasPrefix(routerPkg, "./modules/") {
		routerPkg = strings.TrimPrefix(routerPkg, "./")
		mod, err := readModulePath(filepath.Join(repoRoot, "go.mod"))
		if err != nil {
			return "", err
		}
		return mod + "/" + routerPkg, nil
	}
	return routerPkg, nil
}

func readModulePath(goModPath string) (string, error) {
	b, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}
	lines := strings.Split(string(b), "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(ln, "module ")), nil
		}
	}
	return "", fmt.Errorf("module path not found in go.mod")
}

var goIdentifierRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validateGoIdentifier(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("router function is empty")
	}
	if !goIdentifierRe.MatchString(name) {
		return fmt.Errorf("invalid router function name %q: must be a valid Go identifier", name)
	}
	return nil
}

func inspectRouter(repoRoot string, routerImport string, routerFunc string) (*applet.TypedRouterDescription, error) {
	if err := validateGoIdentifier(routerFunc); err != nil {
		return nil, err
	}

	tmpBase := filepath.Join(repoRoot, "tmp")
	if err := os.MkdirAll(tmpBase, 0o755); err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp(tmpBase, "applet-rpc-typegen-")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "warning: failed to remove temp dir:", err)
		}
	}()

	mainPath := filepath.Join(tmpDir, "main.go")
	mod, err := readModulePath(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		return nil, err
	}
	code := buildRouterInspectorProgram(mod, routerImport, routerFunc)

	if err := os.WriteFile(mainPath, []byte(code), 0o644); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", tmpDir)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=auto")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("router inspection timed out")
		}
		return nil, fmt.Errorf("router inspection failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var desc applet.TypedRouterDescription
	if err := json.Unmarshal(stdout.Bytes(), &desc); err != nil {
		return nil, fmt.Errorf("failed to parse router description: %w", err)
	}
	return &desc, nil
}

func buildRouterInspectorProgram(modulePath string, routerImport string, routerFunc string) string {
	return fmt.Sprintf(`package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"%s/pkg/applet"
	rpc "%s"
)

const routerFuncName = %q

func fail(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func main() {
	factory := reflect.ValueOf(rpc.%s)
	if factory.Kind() != reflect.Func {
		fail("router symbol %%q is not a function", routerFuncName)
	}

	args := make([]reflect.Value, factory.Type().NumIn())
	for i := 0; i < factory.Type().NumIn(); i++ {
		args[i] = reflect.Zero(factory.Type().In(i))
	}

	out := factory.Call(args)
	if len(out) == 0 {
		fail("router function %%q returned no values", routerFuncName)
	}

	if len(out) == 2 {
		errorType := reflect.TypeOf((*error)(nil)).Elem()
		if out[1].Type() != errorType {
			fail("router function %%q second return must be error, got %%s", routerFuncName, out[1].Type())
		}
		if !out[1].IsNil() {
			fail("router function %%q returned error: %%v", routerFuncName, out[1].Interface())
		}
	} else if len(out) != 1 {
		fail("router function %%q returned %%d values; expected 1 or 2", routerFuncName, len(out))
	}

	routerValue := out[0]
	if !routerValue.IsValid() {
		fail("router function %%q returned invalid value", routerFuncName)
	}
	if routerValue.Kind() == reflect.Pointer && routerValue.IsNil() {
		fail("router function %%q returned nil *applet.TypedRPCRouter", routerFuncName)
	}

	router, ok := routerValue.Interface().(*applet.TypedRPCRouter)
	if !ok {
		fail("router function %%q returned %%T; expected *applet.TypedRPCRouter", routerFuncName, routerValue.Interface())
	}

	d, err := applet.DescribeTypedRPCRouter(router)
	if err != nil {
		fail("failed to describe typed rpc router: %%v", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(d)
}
`, modulePath, routerImport, routerFunc, routerFunc)
}
