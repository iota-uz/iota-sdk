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
	var out struct {
		UUID string `json:"uuid"`
		Sha  string `json:"sha"`
	}
	if err := json.Unmarshal(res.Output, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.UUID) != 36 || out.Sha != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("output %s", res.Output)
	}
}

func TestHostBridgeIsHidden(t *testing.T) {
	res := runCode(t, `declare const globalThis: any;
export default async () => ({ outcome: "done", output: [typeof globalThis.host, typeof globalThis.__codeRun] })`, RunInput{})
	if res.Err != nil || string(res.Output) != `["undefined","function"]` {
		t.Fatalf("got %+v %s", res.Err, res.Output)
	}
}

// The Go router is the boundary of trust: these are the calls hostile code would make if it reached the
// bridge, so they are checked without the driver.
func TestRouterValidatesEveryCall(t *testing.T) {
	ctx := context.Background()
	done := 0
	st := newRunState(RunInput{}, Limits{}.WithDefaults(), NewMasker(nil), func() { done++ })
	if _, err := st.report(ctx, json.RawMessage(`{"outcome":"done","output":"1"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.report(ctx, json.RawMessage(`{"outcome":"done","output":"2"}`)); err == nil {
		t.Fatal("a second result was accepted")
	}
	if done != 1 || *st.reported().Output != "1" {
		t.Fatalf("result %v, done %d", st.reported(), done)
	}
	for name, call := range map[string]func() (json.RawMessage, error){
		"log not a list":  func() (json.RawMessage, error) { return st.log(ctx, json.RawMessage(`"x"`)) },
		"digest md5":      func() (json.RawMessage, error) { return st.digest(ctx, json.RawMessage(`{"alg":"MD5","data":"x"}`)) },
		"body unknown id": func() (json.RawMessage, error) { return st.body(ctx, json.RawMessage(`7`)) },
		"result garbage":  func() (json.RawMessage, error) { return st.report(ctx, json.RawMessage(`[`)) },
	} {
		if _, err := call(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	for params, want := range map[string]string{
		`{"url":"https://example.com","method":"TRACE"}`:                   "unsupported method",
		`{"url":"https://example.com","redirect":"sometimes"}`:             "redirect must be",
		`{"url":"https://example.com","headers":{"Host":"x"}}`:             "cannot be set",
		`{"url":"https://example.com","headers":{"Bad Name":"x"}}`:         "invalid header",
		`{"url":"https://example.com","headers":{"X":"a\r\nInjected: 1"}}`: "invalid header",
		`{"url":"https://example.com","body":"x"}`:                         "cannot have a body",
		`{"url":""}`: "invalid URL",
		`[1]`:        "invalid fetch arguments",
	} {
		reply, err := st.fetch(ctx, json.RawMessage(params))
		if err != nil || !strings.Contains(string(reply), want) {
			t.Errorf("%s: got %s %v, want %q", params, reply, err, want)
		}
	}
}

func TestTenantConcurrencyIsBounded(t *testing.T) {
	eng, err := NewEngine(context.Background(), WithMaxConcurrent(4), WithTenantConcurrency(1))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = eng.Close(context.Background()) }()
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

func TestUserErrorsAreNotLimitFailures(t *testing.T) {
	for _, msg := range []string{"upstream: stack overflow in parser", "out of memory", "interrupted"} {
		res := runCode(t, `export default async () => { throw new Error("`+msg+`") }`, RunInput{})
		wantReason(t, res, ReasonException)
		if res.Err.Line != 1 {
			t.Fatalf("%s: no position: %+v", msg, res.Err)
		}
	}
}

func TestUnknownOutcomeMessageIsMasked(t *testing.T) {
	res := runCode(t, `export default async (_: any, env: any) => ({ outcome: env.secrets.K })`,
		RunInput{Secrets: map[string]string{"K": "s3cr\x01t x"}})
	wantReason(t, res, ReasonUnknownOutcome)
	if strings.Contains(res.Err.Message, "s3cr") {
		t.Fatalf("secret in %q", res.Err.Message)
	}
}

func TestFailedIsNotACodeOutcome(t *testing.T) {
	c := mustCompile(t, LangJS, `export default async () => ({ outcome: "failed" })`)
	if _, err := testEngine(t).Run(context.Background(), c, RunInput{Outcomes: []string{"done", "failed"}}); err == nil {
		t.Fatal("Outcomes with failed accepted")
	}
}

func TestTimeoutMessageNamesTheEffectiveDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	c := mustCompile(t, LangJS, `export default async () => { for (;;) {} }`)
	res, err := testEngine(t).Run(ctx, c, RunInput{Outcomes: []string{"done"}})
	if err != nil {
		t.Fatal(err)
	}
	wantReason(t, res, ReasonTimeout)
	if strings.Contains(res.Err.Message, "30s") {
		t.Fatalf("message %q", res.Err.Message)
	}
}
