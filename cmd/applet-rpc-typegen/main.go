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
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/cmd/applet-rpc-typegen/internal/typegen"
	"github.com/iota-uz/iota-sdk/pkg/applet"
)

func main() {
	var routerPkg string
	var outPath string
	var typeName string

	flag.StringVar(&routerPkg, "router-pkg", "", "Router package import path or module-relative path (e.g. modules/bichat/rpc)")
	flag.StringVar(&outPath, "out", "", "Output TypeScript file path")
	flag.StringVar(&typeName, "type-name", "", "Root TypeScript RPC type name (e.g. BichatRPC)")
	flag.Parse()

	if strings.TrimSpace(routerPkg) == "" || strings.TrimSpace(outPath) == "" || strings.TrimSpace(typeName) == "" {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: applet-rpc-typegen --router-pkg <pkg> --out <file> --type-name <TypeName>")
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

	desc, err := inspectRouter(root, routerImport)
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

func inspectRouter(repoRoot string, routerImport string) (*applet.TypedRouterDescription, error) {
	tmpBase := filepath.Join(repoRoot, "tmp")
	if err := os.MkdirAll(tmpBase, 0o755); err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp(tmpBase, "applet-rpc-typegen-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	mainPath := filepath.Join(tmpDir, "main.go")
	mod, err := readModulePath(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		return nil, err
	}
	code := fmt.Sprintf(`package main

import (
  "encoding/json"
  "os"

  "%s/pkg/applet"
  rpc "%s"
)

func main() {
  d, err := applet.DescribeTypedRPCRouter(rpc.Router())
  if err != nil {
    panic(err)
  }
  enc := json.NewEncoder(os.Stdout)
  enc.SetEscapeHTML(false)
  _ = enc.Encode(d)
}
`, mod, routerImport)

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
