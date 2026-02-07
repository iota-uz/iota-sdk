package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type rpcContractConfig struct {
	Name          string
	TypeName      string
	RouterPackage string
	RouterFunc    string
	TargetOut     string

	SDKOut    string
	ModuleOut string
}

var appletNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

func validateAppletName(name string) error {
	if name == "" {
		return fmt.Errorf("missing required --name")
	}
	if !appletNamePattern.MatchString(name) {
		return fmt.Errorf("invalid applet name: %s", name)
	}
	return nil
}

func typeNameFromAppletName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "RPC"
	}

	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' })
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		if len(p) > 1 {
			b.WriteString(p[1:])
		}
	}
	b.WriteString("RPC")
	return b.String()
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

func bichatReexportContent(typeName string) string {
	return "// Re-export canonical RPC contract from @iota-uz/sdk package.\n" +
		fmt.Sprintf("export type { %s } from '@iota-uz/sdk/bichat'\n", typeName)
}

func setEnv(env []string, key, value string) []string {
	out := make([]string, 0, len(env)+1)
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			continue
		}
		out = append(out, e)
	}
	out = append(out, prefix+value)
	return out
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return st.IsDir()
}
