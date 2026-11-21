package code

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path"
	"runtime/debug"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	esbuild "github.com/evanw/esbuild/pkg/api"
	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// driverVersion changes whenever the shape of Compiled.Script or what the driver expects of it changes.
const driverVersion = "code/1"

// compilerVersion is part of every Compiled.Hash: the esbuild version and the driver version.
var compilerVersion = func() string {
	v := "unknown"
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if dep.Path == "github.com/evanw/esbuild" {
				v = dep.Version
				if dep.Replace != nil {
					v = dep.Replace.Version
				}
			}
		}
	}
	return "esbuild@" + v + "+" + driverVersion
}()

// CompilerOption configures NewCompiler.
type CompilerOption func(*compiler)

// WithCache keeps up to size compiled scripts in an LRU cache keyed by Compiled.Hash. size ≤ 0 disables it.
func WithCache(size int) CompilerOption {
	return func(c *compiler) {
		if size <= 0 {
			c.cache = nil
			return
		}
		cache, err := lru.New[string, *Compiled](size)
		if err == nil {
			c.cache = cache
		}
	}
}

type compiler struct {
	cache *lru.Cache[string, *Compiled]
}

// NewCompiler returns a Compiler. It is safe for concurrent use.
func NewCompiler(opts ...CompilerOption) Compiler {
	c := &compiler{}
	for _, o := range opts {
		o(c)
	}
	return c
}

// sourceHash is the Compiled.Hash of a source.
func sourceHash(src Source, filename string) string {
	h := sha256.New()
	for _, part := range []string{compilerVersion, string(src.Lang), filename, src.Code} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (c *compiler) Compile(ctx context.Context, src Source) (*Compiled, error) {
	const op = serrors.Op("automation.code.Compiler.Compile")
	if err := ctx.Err(); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	var loader esbuild.Loader
	switch src.Lang {
	case LangTS:
		loader = esbuild.LoaderTS
	case LangJS:
		loader = esbuild.LoaderJS
	default:
		return nil, &CompileError{Message: `unknown language "` + string(src.Lang) + `": "ts" or "js"`}
	}
	if n := utf8.RuneCountInString(src.Code); n > MaxSourceChars {
		return nil, &CompileError{Message: "the code is too long: at most 100000 characters"}
	}
	if !utf8.ValidString(src.Code) {
		return nil, &CompileError{Message: "the code is not valid UTF-8"}
	}
	filename := path.Base(strings.ReplaceAll(src.Filename, "\\", "/"))
	if filename == "" || filename == "." || filename == "/" {
		filename = "code." + string(src.Lang)
	}
	hash := sourceHash(src, filename)
	if c.cache != nil {
		if hit, ok := c.cache.Get(hash); ok {
			return hit, nil
		}
	}

	res := esbuild.Build(esbuild.BuildOptions{
		Stdin: &esbuild.StdinOptions{
			Contents:   src.Code,
			Sourcefile: filename,
			Loader:     loader,
		},
		Bundle:     true,
		Write:      false,
		Format:     esbuild.FormatIIFE,
		GlobalName: "__mod",
		Target:     esbuild.ES2020,
		Platform:   esbuild.PlatformNeutral,
		Sourcemap:  esbuild.SourceMapExternal,
		Outfile:    "__code.js",
		Charset:    esbuild.CharsetASCII,
		LogLevel:   esbuild.LogLevelSilent,
		Plugins:    []esbuild.Plugin{noImports},
	})
	if len(res.Errors) > 0 {
		return nil, compileError(res.Errors)
	}
	out := &Compiled{Hash: hash}
	for _, f := range res.OutputFiles {
		switch {
		case strings.HasSuffix(f.Path, ".js.map"):
			out.SourceMap = f.Contents
		case strings.HasSuffix(f.Path, ".js"):
			out.Script = string(f.Contents)
		}
	}
	if out.Script == "" {
		return nil, serrors.Wrap(op, errNoOutput)
	}
	if c.cache != nil {
		c.cache.Add(hash, out)
	}
	return out, nil
}

var errNoOutput = errors.New("esbuild produced no script")

// noImports fails the build on every import, require and literal dynamic import: the sandbox has no module
// loader and the code must be self-contained. A computed import() is left to the sandbox, which rejects it.
var noImports = esbuild.Plugin{
	Name: "automation-code-no-imports",
	Setup: func(b esbuild.PluginBuild) {
		b.OnResolve(esbuild.OnResolveOptions{Filter: ".*"}, func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
			return esbuild.OnResolveResult{Errors: []esbuild.Message{{
				Text: `imports are not available in Code nodes: "` + args.Path + `"`,
			}}}, nil
		})
	},
}

func compileError(msgs []esbuild.Message) *CompileError {
	m := msgs[0]
	ce := &CompileError{Message: m.Text}
	if m.Location != nil {
		ce.Line = m.Location.Line
		ce.Column = utf16Column(m.Location.LineText, m.Location.Column) + 1
	}
	if len(msgs) > 1 {
		ce.Message += " (and more errors)"
	}
	return ce
}

// utf16Column converts esbuild's 0-based byte column into a 0-based UTF-16 column.
func utf16Column(line string, byteCol int) int {
	if byteCol > len(line) {
		byteCol = len(line)
	}
	if byteCol < 0 {
		return 0
	}
	n := 0
	for _, r := range line[:byteCol] {
		n += len(utf16.Encode([]rune{r}))
	}
	return n
}
