package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const defaultRouterFunc = "Router"

var (
	appletNamePattern            = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
	exactVersionPattern          = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	localSDKPackageLockPattern   = regexp.MustCompile(`@iota-uz/sdk@(file|link|workspace):`)
	localSDKSpecifierLockPattern = regexp.MustCompile(`(?s)(?:'@iota-uz/sdk'|\"@iota-uz/sdk\"|@iota-uz/sdk):\s*\n\s*specifier:\s*(file|link|workspace):`)
)

type rpcContractConfig struct {
	Name          string
	TypeName      string
	RouterPackage string
	RouterFunc    string
	TargetOut     string
	SDKOut        string
	ModuleOut     string
}

type packageDeps struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return fmt.Errorf("missing subcommand")
	}

	switch args[0] {
	case "rpc":
		return runRPC(args[1:], stdout, stderr)
	case "deps":
		return runDeps(args[1:], stdout, stderr)
	case "gen", "check", "deps-check":
		return runLegacy(args, stdout, stderr)
	default:
		printUsage(stderr)
		return fmt.Errorf("unknown subcommand: %s", args[0])
	}
}

func runLegacy(args []string, stdout io.Writer, stderr io.Writer) error {
	switch args[0] {
	case "gen", "check":
		return runRPC(args, stdout, stderr)
	case "deps-check":
		combined := []string{"check"}
		if len(args) > 1 {
			combined = append(combined, args[1:]...)
		}
		return runDeps(combined, stdout, stderr)
	default:
		return fmt.Errorf("unknown legacy subcommand: %s", args[0])
	}
}

func runRPC(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return fmt.Errorf("missing rpc subcommand")
	}

	subcommand := args[0]
	if subcommand != "gen" && subcommand != "check" {
		printUsage(stderr)
		return fmt.Errorf("unknown rpc subcommand: %s", subcommand)
	}

	cmdFlags := flag.NewFlagSet(subcommand, flag.ContinueOnError)
	cmdFlags.SetOutput(stderr)

	var name string
	var routerFunc string
	cmdFlags.StringVar(&name, "name", "", "Applet name")
	cmdFlags.StringVar(&routerFunc, "router-func", defaultRouterFunc, "Router factory function name in applet rpc package")

	if err := cmdFlags.Parse(args[1:]); err != nil {
		return err
	}
	if cmdFlags.NArg() > 0 {
		printUsage(stderr)
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(cmdFlags.Args(), " "))
	}

	if err := validateAppletName(name); err != nil {
		printUsage(stderr)
		return err
	}

	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	cfg, err := buildRPCConfig(root, name, routerFunc)
	if err != nil {
		return err
	}

	switch subcommand {
	case "gen":
		if err := runGenerate(root, cfg); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "RPC contract generated: %s\n", cfg.Name)
		return nil
	case "check":
		if err := runCheck(root, cfg); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "RPC contract is up to date: %s\n", cfg.Name)
		return nil
	default:
		return errors.New("unreachable")
	}
}

func runDeps(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return fmt.Errorf("missing deps subcommand")
	}

	subcommand := args[0]
	if subcommand != "check" {
		printUsage(stderr)
		return fmt.Errorf("unknown deps subcommand: %s", subcommand)
	}

	cmdFlags := flag.NewFlagSet(subcommand, flag.ContinueOnError)
	cmdFlags.SetOutput(stderr)
	if err := cmdFlags.Parse(args[1:]); err != nil {
		return err
	}
	if cmdFlags.NArg() > 0 {
		printUsage(stderr)
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(cmdFlags.Args(), " "))
	}

	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	return runDepsCheck(root, stdout, stderr)
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  applet rpc gen --name <applet> [--router-func Router]")
	_, _ = fmt.Fprintln(w, "  applet rpc check --name <applet> [--router-func Router]")
	_, _ = fmt.Fprintln(w, "  applet deps check")
}

func validateAppletName(name string) error {
	if name == "" {
		return fmt.Errorf("missing required --name")
	}
	if !appletNamePattern.MatchString(name) {
		return fmt.Errorf("invalid applet name: %s", name)
	}
	return nil
}

func buildRPCConfig(root, name, routerFunc string) (rpcContractConfig, error) {
	typeName := typeNameFromAppletName(name)
	if typeName == "RPC" {
		return rpcContractConfig{}, fmt.Errorf("failed to derive type name from applet name: %s", name)
	}

	sdkDataDir := filepath.Join(root, "ui", "src", name, "data")
	sdkOut := filepath.Join("ui", "src", name, "data", "rpc.generated.ts")
	moduleOut := filepath.Join("modules", name, "presentation", "web", "src", "rpc.generated.ts")
	targetOut := moduleOut
	if isDir(sdkDataDir) {
		targetOut = sdkOut
	}

	return rpcContractConfig{
		Name:          name,
		TypeName:      typeName,
		RouterPackage: filepath.ToSlash(filepath.Join("modules", name, "rpc")),
		RouterFunc:    routerFunc,
		TargetOut:     filepath.ToSlash(targetOut),
		SDKOut:        filepath.ToSlash(sdkOut),
		ModuleOut:     filepath.ToSlash(moduleOut),
	}, nil
}

func runGenerate(root string, cfg rpcContractConfig) error {
	if err := ensureParentDir(root, cfg.TargetOut); err != nil {
		return err
	}

	if err := runTypegen(root, cfg, filepath.Join(root, cfg.TargetOut)); err != nil {
		return err
	}

	if cfg.Name == "bichat" {
		moduleAbs := filepath.Join(root, cfg.ModuleOut)
		if err := ensureParentDir(root, cfg.ModuleOut); err != nil {
			return err
		}
		if err := os.WriteFile(moduleAbs, []byte(bichatReexportContent(cfg.TypeName)), 0o644); err != nil {
			return fmt.Errorf("write bichat re-export shim: %w", err)
		}
	}

	return nil
}

func runCheck(root string, cfg rpcContractConfig) error {
	targetAbs := filepath.Join(root, cfg.TargetOut)
	if _, err := os.Stat(targetAbs); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("RPC target file does not exist: %s\nRun: just applet rpc-gen %s", cfg.TargetOut, cfg.Name)
		}
		return fmt.Errorf("stat target file: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "applet-rpc-contract-*.ts")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	defer os.Remove(tmpPath)

	if err := runTypegen(root, cfg, tmpPath); err != nil {
		return err
	}

	targetBytes, err := os.ReadFile(targetAbs)
	if err != nil {
		return fmt.Errorf("read target file: %w", err)
	}
	tmpBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("read generated temp file: %w", err)
	}
	if !bytes.Equal(targetBytes, tmpBytes) {
		return fmt.Errorf("RPC contract drift detected for applet: %s\nRun: just applet rpc-gen %s", cfg.Name, cfg.Name)
	}

	if cfg.Name == "bichat" {
		moduleAbs := filepath.Join(root, cfg.ModuleOut)
		if _, err := os.Stat(moduleAbs); err == nil {
			actual, readErr := os.ReadFile(moduleAbs)
			if readErr != nil {
				return fmt.Errorf("read bichat re-export shim: %w", readErr)
			}
			expected := bichatReexportContent(cfg.TypeName)
			if string(actual) != expected {
				return fmt.Errorf("BiChat module rpc.generated.ts must be a re-export shim.\nRun: just applet rpc-gen %s", cfg.Name)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat bichat re-export shim: %w", err)
		}
	}

	return nil
}

func runTypegen(root string, cfg rpcContractConfig, outputPath string) error {
	args := []string{
		"run",
		"./cmd/applet-rpc-typegen",
		"--router-pkg", cfg.RouterPackage,
		"--router-func", cfg.RouterFunc,
		"--out", outputPath,
		"--type-name", cfg.TypeName,
	}

	cmd := exec.Command("go", args...)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = setEnv(cmd.Env, "GOTOOLCHAIN", "auto")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run applet-rpc-typegen: %w", err)
	}
	return nil
}

func runDepsCheck(root string, stdout io.Writer, stderr io.Writer) error {
	violations, found, err := checkAppletDeps(root)
	if err != nil {
		return err
	}

	if !found {
		_, _ = fmt.Fprintln(stdout, "No applet web package.json files found.")
		return nil
	}

	if len(violations) > 0 {
		for _, violation := range violations {
			_, _ = fmt.Fprintln(stderr, violation)
		}
		return errors.New("applet SDK dependency policy check failed")
	}

	_, _ = fmt.Fprintln(stdout, "Applet SDK dependency policy check passed.")
	return nil
}

func checkAppletDeps(root string) ([]string, bool, error) {
	modulesDir := filepath.Join(root, "modules")
	violations := make([]string, 0)
	found := false

	err := filepath.WalkDir(modulesDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "/presentation/web/package.json") {
			return nil
		}

		found = true
		packageViolations, err := checkAppletPackage(path)
		if err != nil {
			return err
		}
		violations = append(violations, packageViolations...)
		return nil
	})
	if err != nil {
		return nil, found, err
	}

	return violations, found, nil
}

func checkAppletPackage(packagePath string) ([]string, error) {
	data, err := os.ReadFile(packagePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.ToSlash(packagePath), err)
	}

	var deps packageDeps
	if err := json.Unmarshal(data, &deps); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.ToSlash(packagePath), err)
	}

	spec := deps.Dependencies["@iota-uz/sdk"]
	if spec == "" {
		spec = deps.DevDependencies["@iota-uz/sdk"]
	}
	if spec == "" {
		return nil, nil
	}

	violations := make([]string, 0)
	packageRel := filepath.ToSlash(packagePath)
	if strings.HasPrefix(spec, "file:") || strings.HasPrefix(spec, "link:") || strings.HasPrefix(spec, "workspace:") {
		violations = append(violations,
			fmt.Sprintf("Error: %s uses local @iota-uz/sdk dependency (%q). Use an exact npm version instead.", packageRel, spec))
	}

	if !exactVersionPattern.MatchString(spec) {
		violations = append(violations,
			fmt.Sprintf("Error: %s must pin @iota-uz/sdk to an exact version, got %q.", packageRel, spec))
	}

	lockfile := filepath.Join(filepath.Dir(packagePath), "pnpm-lock.yaml")
	lockViolations, err := checkLockfile(lockfile)
	if err != nil {
		return nil, err
	}
	violations = append(violations, lockViolations...)

	return violations, nil
}

func checkLockfile(lockfilePath string) ([]string, error) {
	_, err := os.Stat(lockfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat %s: %w", filepath.ToSlash(lockfilePath), err)
	}

	data, err := os.ReadFile(lockfilePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.ToSlash(lockfilePath), err)
	}

	content := string(data)
	if localSDKPackageLockPattern.MatchString(content) || localSDKSpecifierLockPattern.MatchString(content) {
		return []string{
			fmt.Sprintf("Error: %s contains local @iota-uz/sdk lock entries. Reinstall dependencies with npm version pinning.", filepath.ToSlash(lockfilePath)),
		}, nil
	}

	return nil, nil
}

func findProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		modPath := filepath.Join(dir, "go.mod")
		data, readErr := os.ReadFile(modPath)
		if readErr == nil {
			if strings.Contains(string(data), "module github.com/iota-uz/iota-sdk") {
				return dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root (go.mod for github.com/iota-uz/iota-sdk)")
		}
		dir = parent
	}
}

func ensureParentDir(root, relPath string) error {
	parent := filepath.Dir(filepath.Join(root, relPath))
	if !isDir(parent) {
		return fmt.Errorf("target directory does not exist: %s", filepath.ToSlash(filepath.Clean(relPath)))
	}
	return nil
}

func typeNameFromAppletName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_'
	})
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		r, size := utf8.DecodeRuneInString(part)
		if r == utf8.RuneError && size == 0 {
			continue
		}
		b.WriteRune(unicode.ToUpper(r))
		b.WriteString(part[size:])
	}
	b.WriteString("RPC")
	return b.String()
}

func bichatReexportContent(typeName string) string {
	return fmt.Sprintf("// Re-export canonical RPC contract from @iota-uz/sdk package.\nexport type { %s } from '@iota-uz/sdk/bichat'\n", typeName)
}

func setEnv(base []string, key, value string) []string {
	env := base
	if env == nil {
		env = os.Environ()
	}
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			if !replaced {
				out = append(out, prefix+value)
				replaced = true
			}
			continue
		}
		out = append(out, entry)
	}
	if !replaced {
		out = append(out, prefix+value)
	}
	return out
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
