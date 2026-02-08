package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	colorReset   = "\033[0m"
	colorCyan    = "\033[36m"
	colorMagenta = "\033[35m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorDim     = "\033[2m"
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

type processSpec struct {
	Name     string
	Command  string
	Args     []string
	Dir      string
	Color    string
	Critical bool
}

type managedProcess struct {
	spec   processSpec
	maxLen int

	mu        sync.Mutex
	cmd       *exec.Cmd
	restartCh chan struct{} // manual restart signal (critical only)
}

var outputMu sync.Mutex

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

	var processes []processSpec

	// Always add templ watcher
	processes = append(processes, processSpec{
		Name: "templ", Command: "templ", Args: []string{"generate", "--watch"},
		Dir: root, Color: colorCyan, Critical: false,
	})

	// Add tailwind if input exists
	tailwindInput := filepath.Join(root, "styles/tailwind/input.css")
	if _, err := os.Stat(tailwindInput); err == nil {
		processes = append(processes, processSpec{
			Name: "css", Command: "pnpm", Dir: root, Color: colorMagenta, Critical: false,
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
	processes = append(processes, processSpec{
		Name: "air", Command: "air", Args: nil,
		Dir: root, Color: colorYellow, Critical: true,
	})

	log.Printf("\n%sr restart air · c clear · q quit%s\n\n", colorDim, colorReset)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	exitCode := runProcesses(ctx, cancel, processes)
	os.Exit(exitCode)
}

func setupApplet(root, appletName string) ([]processSpec, error) {
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
		"APPLET_ASSETS_BASE":                          applet.BasePath + "/assets/",
		"APPLET_VITE_PORT":                            fmt.Sprintf("%d", applet.VitePort),
		fmt.Sprintf("IOTA_APPLET_DEV_%s", upperName):  "1",
		fmt.Sprintf("IOTA_APPLET_VITE_URL_%s", upperName): fmt.Sprintf("http://localhost:%d", applet.VitePort),
		fmt.Sprintf("IOTA_APPLET_ENTRY_%s", upperName):   applet.EntryModule,
		fmt.Sprintf("IOTA_APPLET_CLIENT_%s", upperName):   "/@vite/client",
	}
	for k, v := range envVars {
		if err := os.Setenv(k, v); err != nil {
			return nil, fmt.Errorf("set env %s: %w", k, err)
		}
	}

	// Build applet CSS before starting Vite (tailwindcss --watch needs TTY, may exit immediately)
	log.Println("Building applet CSS...")
	if err := runCommand(context.Background(), viteDir, "pnpm", "exec", "tailwindcss",
		"-i", "src/index.css", "-o", "dist/style.css"); err != nil {
		return nil, fmt.Errorf("applet CSS build failed: %w", err)
	}

	log.Printf("Applet: %s\n", applet.Name)
	log.Printf("URL:    http://localhost:%s%s\n", iotaPort, applet.BasePath)

	return []processSpec{
		{
			Name: "sdk", Command: "pnpm",
			Args: []string{"exec", "tsup", "--config", "tsup.dev.config.ts", "--watch"},
			Dir:  root, Color: colorBlue, Critical: false,
		},
		{
			Name: "acss", Command: "pnpm",
			Args: []string{"-C", viteDir, "exec", "tailwindcss",
				"-i", "src/index.css", "-o", "dist/style.css", "--watch"},
			Dir: root, Color: colorMagenta, Critical: false,
		},
		{
			Name: "vite", Command: "pnpm",
			Args: []string{"-C", viteDir, "exec", "vite"},
			Dir:  root, Color: colorGreen, Critical: true,
		},
	}, nil
}

// --- Process management ---

func (m *managedProcess) runCritical(ctx context.Context, exitCh chan<- string) {
	m.restartCh = make(chan struct{}, 1)

	for {
		cmd, err := startProcess(ctx, m.spec, m.maxLen)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start %s: %v\n", m.spec.Name, err)
			exitCh <- m.spec.Name
			return
		}
		m.mu.Lock()
		m.cmd = cmd
		m.mu.Unlock()

		if waitErr := cmd.Wait(); waitErr != nil {
			outputMu.Lock()
			fmt.Fprintf(os.Stderr, "%s%s[%-*s]%s process exit: %v\n", m.spec.Color, colorDim, m.maxLen, m.spec.Name, colorReset, waitErr)
			outputMu.Unlock()
		}

		if ctx.Err() != nil {
			return
		}

		// Was this a manual restart?
		select {
		case <-m.restartCh:
			prefix := fmt.Sprintf("[%-*s]", m.maxLen, m.spec.Name)
			outputMu.Lock()
			log.Printf("%s%s%s restarting...\n", m.spec.Color, prefix, colorReset)
			outputMu.Unlock()
			continue
		default:
			exitCh <- m.spec.Name
			return
		}
	}
}

func (m *managedProcess) runAuxiliary(ctx context.Context) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		start := time.Now()
		cmd, err := startProcess(ctx, m.spec, m.maxLen)
		if err != nil {
			prefix := fmt.Sprintf("[%-*s]", m.maxLen, m.spec.Name)
			outputMu.Lock()
			fmt.Fprintf(os.Stderr, "%s%s%s failed to start: %v\n",
				m.spec.Color, prefix, colorReset, err)
			outputMu.Unlock()
		} else {
			m.mu.Lock()
			m.cmd = cmd
			m.mu.Unlock()

			waitErr := cmd.Wait()

			if ctx.Err() != nil {
				return
			}

			// Clean exit (code 0): don't restart
			if waitErr == nil {
				prefix := fmt.Sprintf("[%-*s]", m.maxLen, m.spec.Name)
				outputMu.Lock()
				fmt.Fprintf(os.Stderr, "%s%s%s finished\n",
					m.spec.Color, prefix, colorReset)
				outputMu.Unlock()
				return
			}

			// Reset backoff if process ran for a while
			if time.Since(start) > 10*time.Second {
				backoff = time.Second
			}
		}

		prefix := fmt.Sprintf("[%-*s]", m.maxLen, m.spec.Name)
		outputMu.Lock()
		fmt.Fprintf(os.Stderr, "%s%s%s crashed, restarting in %s...\n",
			m.spec.Color, prefix, colorReset, backoff)
		outputMu.Unlock()

		select {
		case <-time.After(backoff):
			backoff = min(backoff*2, maxBackoff)
		case <-ctx.Done():
			return
		}
	}
}

func (m *managedProcess) restart() {
	if m.restartCh != nil {
		select {
		case m.restartCh <- struct{}{}:
		default:
		}
	}
	m.stop()
}

func (m *managedProcess) stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil && m.cmd.Process != nil {
		_ = syscall.Kill(-m.cmd.Process.Pid, syscall.SIGTERM)
	}
}

func (m *managedProcess) forceKill() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil && m.cmd.Process != nil {
		_ = syscall.Kill(-m.cmd.Process.Pid, syscall.SIGKILL)
	}
}

func runProcesses(ctx context.Context, cancel context.CancelFunc, specs []processSpec) int {
	maxLen := 0
	for _, s := range specs {
		if len(s.Name) > maxLen {
			maxLen = len(s.Name)
		}
	}

	managed := make([]*managedProcess, 0, len(specs))
	criticalExitCh := make(chan string, len(specs))
	var wg sync.WaitGroup

	for _, spec := range specs {
		mp := &managedProcess{spec: spec, maxLen: maxLen}
		managed = append(managed, mp)

		wg.Add(1)
		if spec.Critical {
			go func(m *managedProcess) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						criticalExitCh <- fmt.Sprintf("%s (panic: %v)", m.spec.Name, r)
					}
				}()
				m.runCritical(ctx, criticalExitCh)
			}(mp)
		} else {
			go func(m *managedProcess) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						prefix := fmt.Sprintf("[%-*s]", m.maxLen, m.spec.Name)
						outputMu.Lock()
						fmt.Fprintf(os.Stderr, "%s%s%s panic: %v\n", m.spec.Color, prefix, colorReset, r)
						outputMu.Unlock()
					}
				}()
				m.runAuxiliary(ctx)
			}(mp)
		}
	}

	// Keyboard input
	restoreTerm := enableCbreak(ctx)
	defer restoreTerm()
	keyCh := make(chan byte, 8)
	go readKeys(keyCh)

	exitCode := 0
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case name := <-criticalExitCh:
			outputMu.Lock()
			fmt.Fprintf(os.Stderr, "\n%s exited. Shutting down.\n", name)
			outputMu.Unlock()
			exitCode = 1
			cancel()
			break loop
		case key := <-keyCh:
			switch key {
			case 'r':
				for _, m := range managed {
					if m.spec.Name == "air" {
						m.restart()
					}
				}
			case 'c':
				outputMu.Lock()
				log.Print("\033[2J\033[H")
				outputMu.Unlock()
			case 'q':
				cancel()
				break loop
			}
		}
	}

	// Graceful shutdown
	for _, m := range managed {
		m.stop()
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		for _, m := range managed {
			m.forceKill()
		}
		<-done
	}

	return exitCode
}

// --- Keyboard input ---

func enableCbreak(ctx context.Context) func() {
	save := exec.CommandContext(ctx, "stty", "-g")
	save.Stdin = os.Stdin
	state, err := save.Output()
	if err != nil {
		return func() {} // not a terminal
	}

	set := exec.CommandContext(ctx, "stty", "cbreak", "-echo")
	set.Stdin = os.Stdin
	if err := set.Run(); err != nil {
		return func() {}
	}

	return func() {
		restore := exec.CommandContext(context.Background(), "stty", strings.TrimSpace(string(state)))
		restore.Stdin = os.Stdin
		if err := restore.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to restore terminal (stty): %v\n", err)
		}
	}
}

func readKeys(ch chan<- byte) {
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return
		}
		ch <- buf[0]
	}
}

// --- Process launching ---

func startProcess(ctx context.Context, spec processSpec, padLen int) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	prefix := fmt.Sprintf("[%-*s]", padLen, spec.Name)
	coloredPrefix := spec.Color + prefix + colorReset

	go logOutput(stdout, coloredPrefix)
	go logOutput(stderr, coloredPrefix)

	return cmd, nil
}

func logOutput(r io.Reader, prefix string) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line
	for scanner.Scan() {
		outputMu.Lock()
		log.Printf("%s %s\n", prefix, scanner.Text())
		outputMu.Unlock()
	}
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

func checkPrerequisites() error {
	required := map[string]string{
		"air":   "go install github.com/air-verse/air@latest",
		"templ": "go install github.com/a-h/templ/cmd/templ@latest",
		"pnpm":  "npm install -g pnpm",
		"node":  "https://nodejs.org/",
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
