package code

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

func TestCompileTypeScript(t *testing.T) {
	c := mustCompile(t, LangTS, `
type Out = { n: number };
export default async function run(ctx: { n: number }): Promise<{ outcome: "done"; output: Out }> {
  const x = ctx.n as number;
  return { outcome: "done", output: { n: x ?? 0 } };
}`)
	if !strings.HasPrefix(c.Script, "var __mod = ") || len(c.SourceMap) == 0 || len(c.Hash) != 64 {
		t.Fatalf("got %+v", c)
	}
	if strings.Contains(c.Script, "type Out") || strings.Contains(c.Script, ": number") {
		t.Fatalf("types not stripped:\n%s", c.Script)
	}
}

func TestCompileDoesNotCheckTypes(t *testing.T) {
	// A type error is the editor's business (#1331); the server only strips types.
	mustCompile(t, LangTS, `const n: number = "text"; export default async () => ({ outcome: "done", output: n })`)
}

func TestCompileErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		src        Source
		line, col  int
		msgContain string
	}{
		"syntax":      {Source{Code: "export default async () => {\n  return {;\n}", Lang: LangTS}, 2, 11, ""},
		"unicode col": {Source{Code: "const s = \"привет\"; const x = ;", Lang: LangJS}, 1, 31, ""},
		"import":      {Source{Code: "\n  import x from 'lodash'\nexport default x", Lang: LangJS}, 2, 17, "lodash"},
		"language":    {Source{Code: "1", Lang: "py"}, 0, 0, "language"},
		"too long":    {Source{Code: strings.Repeat("x", MaxSourceChars+1), Lang: LangJS}, 0, 0, "too long"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewCompiler().Compile(context.Background(), tc.src)
			var ce *CompileError
			if !errors.As(err, &ce) {
				t.Fatalf("want CompileError, got %v", err)
			}
			if ce.Line != tc.line || ce.Column != tc.col || !strings.Contains(ce.Message, tc.msgContain) {
				t.Fatalf("got %d:%d %q", ce.Line, ce.Column, ce.Message)
			}
			if ce.RunError().Reason != ReasonCompileFailed {
				t.Fatal("RunError reason")
			}
		})
	}
	// Exactly MaxSourceChars characters, multi-byte, is accepted.
	code := "export default 1; //" + strings.Repeat("я", MaxSourceChars-20)
	if _, err := NewCompiler().Compile(context.Background(), Source{Code: code, Lang: LangJS}); err != nil {
		t.Fatal(err)
	}
}

func TestCompileCacheAndHash(t *testing.T) {
	cc := NewCompiler(WithCache(8))
	src := Source{Code: "export default 1", Lang: LangJS}
	a, err := cc.Compile(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := cc.Compile(context.Background(), src)
	if a != b {
		t.Fatal("cache miss for the same source")
	}
	other, _ := cc.Compile(context.Background(), Source{Code: "export default 1", Lang: LangTS})
	if other.Hash == a.Hash {
		t.Fatal("hash ignores the language")
	}
	if c, _ := NewCompiler().Compile(context.Background(), src); c.Hash != a.Hash {
		t.Fatal("hash is not stable")
	}
}

func TestDeclarations(t *testing.T) {
	scope := expr.Scope{Vars: map[string]expr.Field{
		"event": expr.ObjectOf(expr.Fields{
			{Name: "kind", Field: expr.StringField},
			{Name: "id", Field: expr.StringField},
			{Name: "amount", Field: expr.DoubleField},
			{Name: "status", Field: expr.EnumOf("open", "closed")},
			{Name: "tags", Field: expr.ListOf(expr.StringField)},
			{Name: "meta", Field: expr.MapOf(expr.DynField)},
			{Name: "due", Field: expr.OptionalOf(expr.TimestampField)},
		}),
		"tenant": expr.ObjectOf(expr.Fields{{Name: "id", Field: expr.StringField}}),
	}}
	d := Declarations(scope, []string{"sent", "skipped", "sent"}, []string{"API_KEY"})
	for _, want := range []string{
		"interface AutomationContext {\n  readonly event: {\n    readonly kind: string;\n",
		`readonly status: "open" | "closed";`,
		"readonly tags: ReadonlyArray<string>;",
		"readonly meta: Readonly<Record<string, any>>;",
		"readonly due?: string /* RFC 3339 */ | null;",
		"  readonly tenant: {\n    readonly id: string;\n  };\n}",
		`type Outcome = "sent" | "skipped";`,
		`type SecretName = "API_KEY";`,
		"type CodeResult<O extends string> = { outcome: O; output?: unknown };",
		"interface CodeEnv {",
	} {
		if !strings.Contains(d, want) {
			t.Errorf("declarations lack %q:\n%s", want, d)
		}
	}
	if empty := Declarations(expr.Scope{}, nil, nil); !strings.Contains(empty, "type Outcome = never;") || !strings.Contains(empty, "interface AutomationContext {\n}") {
		t.Fatalf("empty declarations:\n%s", empty)
	}
}
