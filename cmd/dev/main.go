package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/devrunner"
)

type appletConfig struct {
	Name        string `json:"name"`
	BasePath    string `json:"basePath"`
	ViteDir     string `json:"viteDir"`
	VitePort    int    `json:"vitePort"`
	EntryModule string `json:"entryModule"`
}

type appletRegistry struct {
	Applets []appletConfig `json:"applets"`
}

type packageDeps struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func main() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)

	var appletName string
	if len(os.Args) >= 2 && os.Args[1] != "" {
		appletName = os.Args[1]
	}

	root, err := findProjectRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := checkPrerequisites(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// Check Go server port
	iotaPort := getEnvOrDefault("IOTA_PORT", "3900")
	iotaPortNum, err := strconv.Atoi(iotaPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid IOTA_PORT: %q\n", iotaPort)
		os.Exit(1)
	}
	if err := checkPort(context.Background(), iotaPortNum, "Go server"); err != nil {
		os.Exit(1)
	}

	var processes []devrunner.ProcessSpec

	// Always add templ watcher
	processes = append(processes, devrunner.ProcessSpec{
		Name: "templ", Command: "templ", Args: []string{"generate", "--watch"},
		Dir: root, Color: devrunner.ColorCyan, Critical: false,
	})

	// Add tailwind if input exists
	tailwindInput := filepath.Join(root, "styles/tailwind/input.css")
	if _, err := os.Stat(tailwindInput); err == nil {
		processes = append(processes, devrunner.ProcessSpec{
			Name: "css", Command: "pnpm", Dir: root, Color: devrunner.ColorMagenta, Critical: false,
			Args: []string{"exec", "tailwindcss", "--input", "styles/tailwind/input.css",
				"--output", "modules/core/presentation/assets/css/main.min.css", "--watch"},
		})
	}

	if appletName != "" {
		appletProcs, err := setupApplet(root, appletName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		processes = append(processes, appletProcs...)
	}

	// Air always runs
	processes = append(processes, devrunner.ProcessSpec{
		Name: "air", Command: "air", Args: nil,
		Dir: root, Color: devrunner.ColorYellow, Critical: true,
	})

	ctx, cancel := devrunner.NotifyContext(context.Background())
	defer cancel()

	runOpts := &devrunner.RunOptions{
		RestartProcessName:       "air",
		ProjectRoot:              root,
		PreflightNodeMajor:       0, // read from package.json
		PreflightPnpm:            true,
		PreflightPackageJSONPath: "package.json",
		PreflightDeps:            appletName != "", // check when running an applet (vite/react)
	}
	exitCode, err := devrunner.Run(ctx, cancel, processes, runOpts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Preflight: %v\n", err)
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func setupApplet(root, appletName string) ([]devrunner.ProcessSpec, error) {
	applet, err := loadAppletConfig(filepath.Join(root, "scripts/applets.json"), appletName)
	if err != nil {
		return nil, err
	}

	viteDir := filepath.Join(root, applet.ViteDir)
	if _, err := os.Stat(viteDir); err != nil {
		return nil, fmt.Errorf("vite directory not found: %s", viteDir)
	}

	// Prebuild
	if _, err := os.Stat(filepath.Join(root, "node_modules")); err != nil {
		log.Println("Installing root dependencies...")
		if err := runCommand(context.Background(), root, "pnpm", "install", "--prefer-frozen-lockfile"); err != nil {
			return nil, fmt.Errorf("failed to install root deps: %w", err)
		}
	}

	if err := buildSDKIfNeeded(root); err != nil {
		return nil, fmt.Errorf("sdk build failed: %w", err)
	}

	if err := refreshAppletDeps(root, viteDir); err != nil {
		return nil, fmt.Errorf("applet dep refresh failed: %w", err)
	}

	// Vite port check
	if err := checkPort(context.Background(), applet.VitePort, "Vite"); err != nil {
		return nil, err
	}

	// Environment
	iotaPort := getEnvOrDefault("IOTA_PORT", "3900")
	upperName := strings.ToUpper(strings.ReplaceAll(applet.Name, "-", "_"))
	envVars := map[string]string{
		"APPLET_ASSETS_BASE":                              applet.BasePath + "/assets/",
		"APPLET_VITE_PORT":                                fmt.Sprintf("%d", applet.VitePort),
		fmt.Sprintf("IOTA_APPLET_DEV_%s", upperName):      "1",
		fmt.Sprintf("IOTA_APPLET_VITE_URL_%s", upperName): fmt.Sprintf("http://localhost:%d", applet.VitePort),
		fmt.Sprintf("IOTA_APPLET_ENTRY_%s", upperName):    applet.EntryModule,
		fmt.Sprintf("IOTA_APPLET_CLIENT_%s", upperName):   "/@vite/client",
	}
	for k, v := range envVars {
		if err := os.Setenv(k, v); err != nil {
			return nil, fmt.Errorf("set env %s: %w", k, err)
		}
	}

	// Write applet-dev.json so the frontend can read base path, port, and backend URL (single source of truth).
	backendURL := fmt.Sprintf("http://localhost:%s", iotaPort)
	manifest := struct {
		BasePath   string `json:"basePath"`
		AssetsBase string `json:"assetsBase"`
		VitePort   int    `json:"vitePort"`
		BackendURL string `json:"backendUrl"`
	}{
		BasePath:   applet.BasePath,
		AssetsBase: applet.BasePath + "/assets/",
		VitePort:   applet.VitePort,
		BackendURL: backendURL,
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		log.Printf("warning: could not marshal applet-dev.json: %v", err)
	} else {
		manifestPath := filepath.Join(viteDir, "applet-dev.json")
		if err := os.WriteFile(manifestPath, manifestBytes, 0644); err != nil {
			log.Printf("warning: could not write %s: %v", manifestPath, err)
		}
	}

	// Applet CSS is compiled by the Vite styles plugin (virtual:applet-styles) on demand, or by the tailwind watch process below.
	log.Printf("Applet: %s\n", applet.Name)
	log.Printf("URL:    http://localhost:%s%s\n", iotaPort, applet.BasePath)

	return []devrunner.ProcessSpec{
		{
			Name: "sdk", Command: "pnpm",
			Args: []string{"exec", "tsup", "--config", "tsup.dev.config.ts", "--watch"},
			Dir:  root, Color: devrunner.ColorBlue, Critical: false,
		},
		{
			Name: "acss", Command: "pnpm",
			Args: []string{"-C", viteDir, "exec", "tailwindcss",
				"-i", "src/index.css", "-o", "dist/style.css", "--watch"},
			Dir: root, Color: devrunner.ColorMagenta, Critical: false,
		},
		{
			Name: "vite", Command: "pnpm",
			Args: []string{"-C", viteDir, "exec", "vite"},
			Dir:  root, Color: devrunner.ColorGreen, Critical: true,
		},
	}, nil
}

// --- Prerequisites & config ---

func findProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root (go.mod)")
		}
		dir = parent
	}
}

// checkPrerequisites verifies air and templ only. Node and pnpm are checked by devrunner preflight (version + remediation).
func checkPrerequisites() error {
	required := map[string]string{
		"air":   "go install github.com/air-verse/air@latest",
		"templ": "go install github.com/a-h/templ/cmd/templ@latest",
	}

	var missing []string
	for cmd, install := range required {
		if _, err := exec.LookPath(cmd); err != nil {
			missing = append(missing, fmt.Sprintf("  %s: %s", cmd, install))
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("missing required tools:\n%s", strings.Join(missing, "\n"))
	}

	return nil
}

func loadAppletConfig(path, name string) (*appletConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read applets.json: %w", err)
	}

	var registry appletRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("failed to parse applets.json: %w", err)
	}

	for _, applet := range registry.Applets {
		if applet.Name == name {
			return &applet, nil
		}
	}

	return nil, fmt.Errorf("unknown applet: %s", name)
}

// --- SDK build ---

func buildSDKIfNeeded(root string) error {
	distIndex := filepath.Join(root, "dist/index.mjs")
	hashFile := filepath.Join(root, "dist/.sdk-build-hash")

	needsBuild := false
	if _, err := os.Stat(distIndex); err != nil {
		needsBuild = true
	} else {
		currentHash, err := computeSDKHash(root)
		if err != nil {
			return err
		}

		savedHash, _ := os.ReadFile(hashFile)
		if string(savedHash) != currentHash {
			needsBuild = true
		}
	}

	if needsBuild {
		log.Println("Building @iota-uz/sdk (tsup, dev mode)...")
		if err := runCommand(context.Background(), root, "pnpm", "run", "build:js:dev"); err != nil {
			return err
		}

		currentHash, err := computeSDKHash(root)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Join(root, "dist"), 0755); err != nil {
			return fmt.Errorf("create dist directory: %w", err)
		}
		if err := os.WriteFile(hashFile, []byte(currentHash), 0644); err != nil {
			return err
		}
	}

	return nil
}

func computeSDKHash(root string) (string, error) {
	uiSrc := filepath.Join(root, "ui/src")
	var files []string

	err := filepath.WalkDir(uiSrc, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".ts" || ext == ".tsx" || ext == ".css" {
				files = append(files, path)
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	// Include build config files that affect output
	for _, name := range []string{"tsup.config.ts", "tsup.dev.config.ts"} {
		p := filepath.Join(root, name)
		if _, err := os.Stat(p); err == nil {
			files = append(files, p)
		}
	}

	sort.Strings(files)

	hasher := sha256.New()
	for _, file := range files {
		relPath, _ := filepath.Rel(root, file)
		hasher.Write([]byte(relPath))

		content, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		hasher.Write(content)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// --- Applet dependencies ---

func refreshAppletDeps(root, viteDir string) error {
	nodeModules := filepath.Join(viteDir, "node_modules")
	didInstall := false

	localSDKDependency, err := hasLocalSDKDependency(viteDir)
	if err != nil {
		return err
	}

	if _, err := os.Stat(nodeModules); err != nil {
		log.Println("Installing applet dependencies...")
		if err := runCommand(context.Background(), root, "pnpm", "-C", viteDir, "install", "--prefer-frozen-lockfile"); err != nil {
			return err
		}
		didInstall = true
	} else if localSDKDependency {
		distIndex := filepath.Join(root, "dist/index.mjs")
		sdkModule := filepath.Join(nodeModules, "@iota-uz/sdk/dist/index.mjs")

		if isNewer(distIndex, sdkModule) {
			log.Println("Refreshing applet deps (local @iota-uz/sdk changed)...")
			if err := runCommand(context.Background(), root, "pnpm", "-C", viteDir, "install", "--prefer-frozen-lockfile"); err != nil {
				return err
			}
			didInstall = true
		}
	}

	viteCache := filepath.Join(nodeModules, ".vite")
	if didInstall {
		if err := os.RemoveAll(viteCache); err != nil {
			log.Printf("warning: failed to clear Vite cache after install: %v", err)
		}
	} else {
		distIndex := filepath.Join(root, "dist/index.mjs")
		if isNewer(distIndex, viteCache) {
			log.Println("Clearing Vite dep cache (SDK bundle changed)...")
			if err := os.RemoveAll(viteCache); err != nil {
				log.Printf("warning: failed to clear Vite cache: %v", err)
			}
		}
	}

	return nil
}

func hasLocalSDKDependency(viteDir string) (bool, error) {
	packageJSONPath := filepath.Join(viteDir, "package.json")
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return false, fmt.Errorf("failed to read applet package.json: %w", err)
	}

	var deps packageDeps
	if err := json.Unmarshal(data, &deps); err != nil {
		return false, fmt.Errorf("failed to parse applet package.json: %w", err)
	}

	spec := deps.Dependencies["@iota-uz/sdk"]
	if spec == "" {
		spec = deps.DevDependencies["@iota-uz/sdk"]
	}

	return strings.HasPrefix(spec, "file:") ||
		strings.HasPrefix(spec, "link:") ||
		strings.HasPrefix(spec, "workspace:"), nil
}

// --- Utilities ---

func isNewer(source, target string) bool {
	srcInfo, err := os.Stat(source)
	if err != nil {
		return false
	}
	tgtInfo, err := os.Stat(target)
	if err != nil {
		return true
	}
	return srcInfo.ModTime().After(tgtInfo.ModTime())
}

func checkPort(ctx context.Context, port int, label string) error {
	addr := fmt.Sprintf(":%d", port)
	lc := net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Port %d is already in use (%s)\n", port, label)
		fmt.Fprintf(os.Stderr, "  Kill it: lsof -ti :%d | xargs kill\n", port)
		return err
	}
	if err := ln.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: close port check listener: %v\n", err)
	}
	return nil
}

func runCommand(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getEnvOrDefault(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}
