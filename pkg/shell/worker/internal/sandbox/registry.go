package sandbox

import (
	"fmt"

	"github.com/ewhauser/gbash"
	"github.com/ewhauser/gbash/commands"
	"github.com/ewhauser/gbash/contrib/awk"
	"github.com/ewhauser/gbash/contrib/jq"
)

// allowedCommands is the fixed set of text and VFS commands visible to the
// guest. Identity, process, network, link and alternative-shell commands are
// deliberately absent; awk and jq come from the pinned contrib modules.
var allowedCommands = []string{
	"base64", "basename", "bash", "cat", "cksum", "column", "comm", "cp", "csplit", "cut",
	"date", "diff", "dirname", "du", "echo", "egrep", "expand", "expr", "false", "fgrep",
	"file", "find", "fmt", "fold", "grep", "gunzip", "gzip", "head", "join", "ls",
	"md5sum", "mkdir", "mktemp", "mv", "nl", "numfmt", "od", "paste", "printf", "pwd",
	"readlink", "realpath", "rev", "rm", "rmdir", "sed", "seq", "sh", "sha1sum",
	"sha256sum", "sha512sum", "shuf", "sort", "split", "stat", "strings", "tac", "tail",
	"tar", "tee", "test", "touch", "tr", "tree", "true", "truncate", "tsort", "unexpand",
	"uniq", "wc", "xargs", "yes", "zcat",
}

func newRegistry() (*commands.Registry, error) {
	builtin := gbash.DefaultRegistry()
	registry := commands.NewRegistry()
	for _, name := range allowedCommands {
		cmd, ok := builtin.Lookup(name)
		if !ok {
			return nil, fmt.Errorf("sandbox: command %q is not provided by gbash", name)
		}
		if err := registry.Register(cmd); err != nil {
			return nil, err
		}
	}
	for _, cmd := range []commands.Command{awk.NewAWK(), jq.NewJQ()} {
		if err := registry.Register(cmd); err != nil {
			return nil, err
		}
	}
	return registry, nil
}
