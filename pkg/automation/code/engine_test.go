package code

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunReturnsOutcomeAndOutput(t *testing.T) {
	res := runCode(t, `
interface Issue { title: string; n: number }
export default async function run(ctx: { issue: Issue }, env: any) {
  env.log("hello", { n: ctx.issue.n }, [1, 2]);
  return { outcome: "done", output: { title: ctx.issue.title.toUpperCase(), twice: ctx.issue.n * 2 } };
}`, RunInput{Ctx: json.RawMessage(`{"issue":{"title":"bug","n":21}}`)})
	if res.Err != nil {
		t.Fatalf("unexpected error: %v\n%s", res.Err, res.Err.Stack)
	}
	if res.Outcome != "done" {
		t.Fatalf("outcome %q", res.Outcome)
	}
	if string(res.Output) != `{"title":"BUG","twice":42}` {
		t.Fatalf("output %s", res.Output)
	}
	if len(res.Logs) != 1 || res.Logs[0] != `hello {"n":21} [1,2]` {
		t.Fatalf("logs %q", res.Logs)
	}
}

func TestRunWithoutOutput(t *testing.T) {
	res := runCode(t, `export default async () => ({ outcome: "done" })`, RunInput{})
	if res.Err != nil || res.Outcome != "done" || res.Output != nil {
		t.Fatalf("got %+v", res)
	}
}

func TestCtxIsDeeplyFrozen(t *testing.T) {
	res := runCode(t, `
export default async function run(ctx: any) {
  const errs: string[] = [];
  for (const f of [() => { ctx.a = 1 }, () => { ctx.nested.b = 2 }, () => { ctx.list.push(3) }]) {
    try { f(); errs.push("mutated") } catch (e) { errs.push((e as Error).name) }
  }
  return { outcome: "done", output: errs };
}`, RunInput{Ctx: json.RawMessage(`{"nested":{"b":1},"list":[1]}`)})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if string(res.Output) != `["TypeError","TypeError","TypeError"]` {
		t.Fatalf("output %s", res.Output)
	}
}

func TestUnknownOutcome(t *testing.T) {
	res := runCode(t, `export default async () => ({ outcome: "nope" })`, RunInput{})
	wantReason(t, res, ReasonUnknownOutcome)
	res = runCode(t, `export default async () => ({ output: 1 })`, RunInput{})
	wantReason(t, res, ReasonUnknownOutcome)
}

func TestOutputInvalid(t *testing.T) {
	for name, code := range map[string]string{
		"bigint":     `export default async () => ({ outcome: "done", output: { id: 10n } })`,
		"cycle":      `export default async () => { const a: any = {}; a.self = a; return { outcome: "done", output: a } }`,
		"function":   `export default async () => ({ outcome: "done", output: () => 1 })`,
		"not object": `export default async () => 42`,
		"too large":  `export default async () => ({ outcome: "done", output: "x".repeat(2000) })`,
	} {
		t.Run(name, func(t *testing.T) {
			res := runCode(t, code, RunInput{Limits: Limits{MaxOutputBytes: 1000}})
			wantReason(t, res, ReasonOutputInvalid)
		})
	}
}

func TestExceptionHasSourcePosition(t *testing.T) {
	code := "type T = { a: number };\n" +
		"function fail(x: T): never {\n" +
		"  throw new RangeError(\"bad value \" + x.a);\n" +
		"}\n" +
		"export default async function run() {\n" +
		"  fail({ a: 7 });\n" +
		"}\n"
	c, err := NewCompiler().Compile(context.Background(), Source{Code: code, Lang: LangTS, Filename: "node.ts"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := testEngine(t).Run(context.Background(), c, RunInput{Outcomes: []string{"done"}})
	if err != nil {
		t.Fatal(err)
	}
	wantReason(t, res, ReasonException)
	if res.Err.Message != "RangeError: bad value 7" {
		t.Fatalf("message %q", res.Err.Message)
	}
	if res.Err.Line != 3 {
		t.Fatalf("line %d col %d, stack:\n%s", res.Err.Line, res.Err.Column, res.Err.Stack)
	}
	if !strings.Contains(res.Err.Stack, "node.ts:3:") || !strings.Contains(res.Err.Stack, "node.ts:6:") {
		t.Fatalf("stack not mapped:\n%s", res.Err.Stack)
	}
	if strings.Contains(res.Err.Stack, "<eval>") || strings.Contains(res.Err.Stack, "<compile>") {
		t.Fatalf("stack leaks sandbox frames:\n%s", res.Err.Stack)
	}
}

func TestTopLevelThrowAndNonErrorThrow(t *testing.T) {
	res := runCode(t, "throw new Error('at load');\nexport default async () => ({ outcome: 'done' })", RunInput{})
	wantReason(t, res, ReasonException)
	if res.Err.Message != "Error: at load" || res.Err.Line != 1 {
		t.Fatalf("got %+v", res.Err)
	}
	res = runCode(t, `export default async () => { throw { code: 42 } }`, RunInput{})
	wantReason(t, res, ReasonException)
	if !strings.Contains(res.Err.Message, `{"code":42}`) {
		t.Fatalf("message %q", res.Err.Message)
	}
}

func TestMissingDefaultExport(t *testing.T) {
	res := runCode(t, `export const run = async () => ({ outcome: "done" })`, RunInput{})
	wantReason(t, res, ReasonException)
	if !strings.Contains(res.Err.Message, "export default") {
		t.Fatalf("message %q", res.Err.Message)
	}
}

func TestPromiseThatNeverSettles(t *testing.T) {
	start := time.Now()
	res := runCode(t, `export default () => new Promise(() => {})`, RunInput{Limits: Limits{Timeout: time.Second}})
	wantReason(t, res, ReasonOutcomeUnknown)
	within(t, time.Since(start), time.Second)
}

func TestSecretMissingBeforeTheCodeRuns(t *testing.T) {
	var sink recordingSink
	res := runCode(t, `declare const host: any; host.rpcSync("log", ["ran"]); export default async () => ({ outcome: "done" })`,
		RunInput{Secrets: map[string]string{"API_KEY": ""}, Logger: &sink})
	wantReason(t, res, ReasonSecretMissing)
	if len(sink.lines()) != 0 {
		t.Fatal("the code ran")
	}
}

func TestInvalidInputIsAHostError(t *testing.T) {
	eng := testEngine(t)
	c := mustCompile(t, LangJS, `export default async () => ({ outcome: "done" })`)
	for name, in := range map[string]RunInput{
		"secret name": {Secrets: map[string]string{"api-key": "x"}},
		"ctx array":   {Ctx: json.RawMessage(`[1]`)},
		"ctx invalid": {Ctx: json.RawMessage(`{`)},
	} {
		if _, err := eng.Run(context.Background(), c, in); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := eng.Run(context.Background(), nil, RunInput{}); err == nil {
		t.Error("nil script: no error")
	}
}

func TestSecretsOnlyDeclared(t *testing.T) {
	res := runCode(t, `export default async (_: any, env: any) => ({ outcome: "done", output: Object.keys(env.secrets) })`,
		RunInput{Secrets: map[string]string{"B_KEY": "1", "A_KEY": "2"}})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	var keys []string
	if err := json.Unmarshal(res.Output, &keys); err != nil || len(keys) != 2 {
		t.Fatalf("output %s", res.Output)
	}
}

func TestLogLimits(t *testing.T) {
	var sink recordingSink
	res := runCode(t, `export default async (_: any, env: any) => {
  for (let i = 0; i < 20; i++) env.log("line", i, "y".repeat(50));
  return { outcome: "done" };
}`, RunInput{Limits: Limits{MaxLogLines: 5, MaxLogLineChars: 10}, Logger: &sink})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(res.Logs) != 6 {
		t.Fatalf("logs %q", res.Logs)
	}
	if res.Logs[0] != "line 0 yyy…" {
		t.Fatalf("first line %q", res.Logs[0])
	}
	if !strings.Contains(res.Logs[5], "dropped") {
		t.Fatalf("no drop notice: %q", res.Logs[5])
	}
	if len(sink.lines()) != 6 {
		t.Fatalf("sink got %d lines", len(sink.lines()))
	}
}

func TestEnvCrypto(t *testing.T) {
	res := runCode(t, `export default async (_: any, env: any) => ({ outcome: "done", output: {
  uuid: env.crypto.randomUUID(),
  sha: await env.crypto.digest("SHA-256", "abc"),
} })`, RunInput{})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	var out struct{ UUID, Sha string }
	if err := json.Unmarshal(res.Output, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.UUID) != 36 || out.Sha != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("output %s", res.Output)
	}
}

func TestResultIsAcceptedOnce(t *testing.T) {
	res := runCode(t, `export default async () => {
  host.rpcSync("__result", { outcome: "done", output: "1" });
  let second = "accepted";
  try { host.rpcSync("__result", { outcome: "done", output: "2" }) } catch (e) { second = "refused" }
  return { outcome: "done", output: second };
}
declare const host: any;`, RunInput{})
	if res.Err != nil || string(res.Output) != "1" {
		t.Fatalf("got %+v %s", res.Err, res.Output)
	}
}

func TestHostCallsValidateParameters(t *testing.T) {
	res := runCode(t, `declare const host: any;
export default async () => {
  const out: string[] = [];
  const tries: [string, any][] = [["log", "not a list"], ["digest", { alg: "MD5", data: "x" }], ["uuid", 1], ["nope", 1]];
  for (const [m, p] of tries) {
    try { host.rpcSync(m, p); out.push(m + ":ok") } catch (e) { out.push(m + ":refused") }
  }
  const f = await host.rpc("fetch", { url: "https://example.com", method: "TRACE" });
  out.push("fetch:" + (f.error ? "refused" : "ok"));
  return { outcome: "done", output: out };
}`, RunInput{})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if string(res.Output) != `["log:refused","digest:refused","uuid:ok","nope:refused","fetch:refused"]` {
		t.Fatalf("output %s", res.Output)
	}
}

func TestTenantConcurrencyIsBounded(t *testing.T) {
	eng, err := NewEngine(context.Background(), WithMaxConcurrent(4), WithTenantConcurrency(1))
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close(context.Background())
	c := mustCompile(t, LangJS, `export default async () => { const end = Date.now() + 150; while (Date.now() < end) {} return { outcome: "done" } }`)
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := eng.Run(context.Background(), c, RunInput{Outcomes: []string{"done"}, Tenant: "t1"})
			if err != nil || res.Err != nil {
				t.Errorf("run: %v %v", err, res.Err)
			}
		}()
	}
	wg.Wait()
	if took := time.Since(start); took < 400*time.Millisecond {
		t.Fatalf("three runs of one tenant overlapped: %s", took)
	}
}

func TestClosedEngine(t *testing.T) {
	eng, err := NewEngine(context.Background(), WithCompilationCacheDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Run(context.Background(), mustCompile(t, LangJS, `export default 1`), RunInput{}); err == nil {
		t.Fatal("run on a closed engine")
	}
}

type recordingSink struct {
	mu  sync.Mutex
	got []string
}

func (s *recordingSink) Log(_, line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.got = append(s.got, line)
}

func (s *recordingSink) lines() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.got...)
}
