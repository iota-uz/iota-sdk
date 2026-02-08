package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iota-uz/iota-sdk/internal/applet/rpccodegen"
)

const defaultRouterFunc = "Router"

var (
	exactVersionPattern          = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	localSDKPackageLockPattern   = regexp.MustCompile(`@iota-uz/sdk@(file|link|workspace):`)
	localSDKSpecifierLockPattern = regexp.MustCompile(`(?s)(?:'@iota-uz/sdk'|\"@iota-uz/sdk\"|@iota-uz/sdk):\s*\n\s*specifier:\s*(file|link|workspace):`)
)

type packageDeps struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// NewAppletCommand creates the applet command group (rpc gen/check, deps check).
func NewAppletCommand() *cobra.Command {
	appletCmd := &cobra.Command{
		Use:   "applet",
		Short: "Applet RPC and dependency utilities",
		Long:  "Generate or check applet RPC contracts and validate applet SDK dependency policy.",
	}

	rpcCmd := &cobra.Command{
		Use:   "rpc",
		Short: "RPC contract generation and validation",
	}
	appletCmd.AddCommand(rpcCmd)

	var name, routerFunc string
	genCmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate RPC contract TypeScript from applet router",
		RunE:  runAppletRPCGen(&name, &routerFunc),
	}
	genCmd.Flags().StringVar(&name, "name", "", "Applet name (e.g. bichat)")
	_ = genCmd.MarkFlagRequired("name")
	genCmd.Flags().StringVar(&routerFunc, "router-func", defaultRouterFunc, "Router factory function name in applet rpc package")
	rpcCmd.AddCommand(genCmd)

	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "Check that RPC contract is up to date",
		RunE:  runAppletRPCCheck(&name, &routerFunc),
	}
	checkCmd.Flags().StringVar(&name, "name", "", "Applet name (e.g. bichat)")
	_ = checkCmd.MarkFlagRequired("name")
	checkCmd.Flags().StringVar(&routerFunc, "router-func", defaultRouterFunc, "Router factory function name in applet rpc package")
	rpcCmd.AddCommand(checkCmd)

	depsCmd := &cobra.Command{
		Use:   "deps",
		Short: "Applet dependency checks",
	}
	appletCmd.AddCommand(depsCmd)

	depsCheckCmd := &cobra.Command{
		Use:   "check",
		Short: "Check applet SDK dependency policy",
		RunE:  runAppletDepsCheck,
	}
	depsCmd.AddCommand(depsCheckCmd)

	return appletCmd
}

func runAppletRPCGen(name, routerFunc *string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := rpccodegen.ValidateAppletName(*name); err != nil {
			return err
		}
		root, err := rpccodegen.FindProjectRoot()
		if err != nil {
			return err
		}
		cfg, err := rpccodegen.BuildRPCConfig(root, *name, *routerFunc)
		if err != nil {
			return err
		}
		if err := rpccodegen.EnsureParentDir(root, cfg.TargetOut); err != nil {
			return err
		}
		targetPath := filepath.Join(root, cfg.TargetOut)
		if err := rpccodegen.RunTypegen(root, cfg, targetPath); err != nil {
			return err
		}
		if cfg.Name == "bichat" {
			moduleAbs := filepath.Join(root, cfg.ModuleOut)
			if err := rpccodegen.EnsureParentDir(root, cfg.ModuleOut); err != nil {
				return err
			}
			if err := os.WriteFile(moduleAbs, []byte(rpccodegen.BichatReexportContent(cfg.TypeName)), 0o644); err != nil {
				return err
			}
		}
		cmd.Println("RPC contract generated:", cfg.Name)
		return nil
	}
}

func runAppletRPCCheck(name, routerFunc *string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := rpccodegen.ValidateAppletName(*name); err != nil {
			return err
		}
		root, err := rpccodegen.FindProjectRoot()
		if err != nil {
			return err
		}
		cfg, err := rpccodegen.BuildRPCConfig(root, *name, *routerFunc)
		if err != nil {
			return err
		}
		targetAbs := filepath.Join(root, cfg.TargetOut)
		if _, err := os.Stat(targetAbs); err != nil {
			if os.IsNotExist(err) {
				return errors.New("RPC target file does not exist: " + cfg.TargetOut + "\nRun: just applet rpc-gen " + cfg.Name)
			}
			return err
		}
		tmpFile, err := os.CreateTemp("", "applet-rpc-contract-*.ts")
		if err != nil {
			return err
		}
		tmpPath := tmpFile.Name()
		if err := tmpFile.Close(); err != nil {
			return err
		}
		defer func() { _ = os.Remove(tmpPath) }()
		if err := rpccodegen.RunTypegen(root, cfg, tmpPath); err != nil {
			return err
		}
		targetBytes, err := os.ReadFile(targetAbs)
		if err != nil {
			return err
		}
		tmpBytes, err := os.ReadFile(tmpPath)
		if err != nil {
			return err
		}
		if !bytes.Equal(targetBytes, tmpBytes) {
			return errors.New("RPC contract drift detected for applet: " + cfg.Name + "\nRun: just applet rpc-gen " + cfg.Name)
		}
		if cfg.Name == "bichat" {
			moduleAbs := filepath.Join(root, cfg.ModuleOut)
			if _, err := os.Stat(moduleAbs); err == nil {
				actual, readErr := os.ReadFile(moduleAbs)
				if readErr != nil {
					return readErr
				}
				expected := rpccodegen.BichatReexportContent(cfg.TypeName)
				if string(actual) != expected {
					return errors.New("BiChat module rpc.generated.ts must be a re-export shim.\nRun: just applet rpc-gen " + cfg.Name)
				}
			} else if !os.IsNotExist(err) {
				return err
			}
		}
		cmd.Println("RPC contract is up to date:", cfg.Name)
		return nil
	}
}

func runAppletDepsCheck(cmd *cobra.Command, args []string) error {
	root, err := rpccodegen.FindProjectRoot()
	if err != nil {
		return err
	}
	violations, found, err := checkAppletDeps(root)
	if err != nil {
		return err
	}
	if !found {
		cmd.Println("No applet web package.json files found.")
		return nil
	}
	if len(violations) > 0 {
		for _, v := range violations {
			_, _ = cmd.ErrOrStderr().Write([]byte(v + "\n"))
		}
		return errors.New("applet SDK dependency policy check failed")
	}
	cmd.Println("Applet SDK dependency policy check passed.")
	return nil
}

func checkAppletDeps(root string) ([]string, bool, error) {
	modulesDir := filepath.Join(root, "modules")
	var violations []string
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
		pv, err := checkAppletPackage(path)
		if err != nil {
			return err
		}
		violations = append(violations, pv...)
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
		return nil, err
	}
	var deps packageDeps
	if err := json.Unmarshal(data, &deps); err != nil {
		return nil, err
	}
	spec := deps.Dependencies["@iota-uz/sdk"]
	if spec == "" {
		spec = deps.DevDependencies["@iota-uz/sdk"]
	}
	if spec == "" {
		return nil, nil
	}
	var violations []string
	packageRel := filepath.ToSlash(packagePath)
	if strings.HasPrefix(spec, "file:") || strings.HasPrefix(spec, "link:") || strings.HasPrefix(spec, "workspace:") {
		violations = append(violations,
			"Error: "+packageRel+" uses local @iota-uz/sdk dependency ("+spec+"). Use an exact npm version instead.")
	}
	if !exactVersionPattern.MatchString(spec) {
		violations = append(violations,
			"Error: "+packageRel+" must pin @iota-uz/sdk to an exact version, got "+spec+".")
	}
	lockfile := filepath.Join(filepath.Dir(packagePath), "pnpm-lock.yaml")
	lv, err := checkLockfile(lockfile)
	if err != nil {
		return nil, err
	}
	violations = append(violations, lv...)
	return violations, nil
}

func checkLockfile(lockfilePath string) ([]string, error) {
	if _, err := os.Stat(lockfilePath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	data, err := os.ReadFile(lockfilePath)
	if err != nil {
		return nil, err
	}
	content := string(data)
	if localSDKPackageLockPattern.MatchString(content) || localSDKSpecifierLockPattern.MatchString(content) {
		return []string{
			"Error: " + filepath.ToSlash(lockfilePath) + " contains local @iota-uz/sdk lock entries. Reinstall dependencies with npm version pinning.",
		}, nil
	}
	return nil, nil
}
