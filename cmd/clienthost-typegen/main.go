package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/pkg/clienthost"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			panic("go.mod not found")
		}
		root = parent
	}
	target := filepath.Join(root, "web", "client-host", "src", "protocol.ts")
	content := fmt.Sprintf("// GENERATED — do not edit\nexport const CLIENT_HOST_PROTOCOL_VERSION = %q\n", clienthost.ProtocolVersion)
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		panic(err)
	}
}
