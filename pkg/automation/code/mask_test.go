package code

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestMaskerForms(t *testing.T) {
	const secret = `s3cr3t "q" & a/b+c?`
	m := NewMasker(map[string]string{"API_KEY": secret, "EMPTY": ""})
	enc, err := json.Marshal(secret)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := json.Marshal(string(enc))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		secret,
		string(enc),
		string(inner),
		url.QueryEscape(secret),
		url.PathEscape(secret),
		encodeURIComponent(secret),
		"prefix " + secret + " suffix",
	} {
		if got := m.Mask(s); strings.Contains(got, "s3cr3t") {
			t.Errorf("%q masked to %q", s, got)
		}
	}
	if got := m.Mask("nothing here"); got != "nothing here" {
		t.Fatalf("masked %q", got)
	}
}

func TestMaskJSONKeepsJSON(t *testing.T) {
	m := NewMasker(map[string]string{"K": "12345", "T": "tok"})
	got := m.MaskJSON(json.RawMessage(`{"tok":"a tok b","n":12345,"m":9123456,"list":[true,null,"x"],"nested":{"q":"%22tok"}}`))
	if !json.Valid(got) {
		t.Fatalf("invalid JSON %s", got)
	}
	want := `{"***":"a *** b","n":"***","m":"9***6","list":[true,null,"x"],"nested":{"q":"%22***"}}`
	if string(got) != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	lits := NewMasker(map[string]string{"A": "true", "B": "null", "C": "false"})
	if got := lits.MaskJSON(json.RawMessage(`{"a":true,"b":null,"c":[false,"x"]}`)); string(got) != `{"a":"***","b":"***","c":["***","x"]}` {
		t.Fatalf("literals not masked: %s", got)
	}
	if got := m.MaskJSON(json.RawMessage(`not json tok`)); string(got) != `"not json ***"` {
		t.Fatalf("got %s", got)
	}
	if got := NewMasker(nil).MaskJSON(json.RawMessage(`{"a":1}`)); string(got) != `{"a":1}` {
		t.Fatalf("got %s", got)
	}
}

// The acceptance scenario: the secret never leaves the sandbox in logs, output, message or stack, in any of
// its forms.
func TestSecretNeverLeavesTheSandbox(t *testing.T) {
	const secret = "s3cr3t"
	in := func() RunInput {
		return RunInput{Secrets: map[string]string{"API_KEY": secret}}
	}
	check := func(t *testing.T, res *RunResult) {
		t.Helper()
		all := strings.Join(res.Logs, "\n") + string(res.Output)
		if res.Err != nil {
			all += res.Err.Message + res.Err.Stack
		}
		if strings.Contains(all, secret) {
			t.Fatalf("secret leaked:\n%s", all)
		}
	}
	res := runCode(t, `export default async (_: any, env: any) => {
  const k = env.secrets.API_KEY;
  env.log(k, { k }, JSON.stringify({ k }), encodeURIComponent(k), [k]);
  return { outcome: "done", output: { k, json: JSON.stringify({ k }), url: "https://x.example/?key=" + encodeURIComponent(k), [k]: 1 } };
}`, in())
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	check(t, res)
	if !strings.Contains(string(res.Output), Mask) || len(res.Logs) != 1 || !strings.Contains(res.Logs[0], Mask) {
		t.Fatalf("nothing masked: %s %q", res.Output, res.Logs)
	}

	res = runCode(t, `export default async (_: any, env: any) => {
  const err = new Error("bad key " + env.secrets.API_KEY);
  err.stack = "at " + env.secrets.API_KEY + " " + err.stack;
  throw err;
}`, in())
	wantReason(t, res, ReasonException)
	check(t, res)

	res = runCode(t, `export default async (_: any, env: any) => { throw env.secrets.API_KEY }`, in())
	wantReason(t, res, ReasonException)
	check(t, res)

	res = runCode(t, `export default async (_: any, env: any) => ({ outcome: env.secrets.API_KEY })`, in())
	wantReason(t, res, ReasonUnknownOutcome)
	check(t, res)

	res = runCode(t, `export default async (_: any, env: any) => ({ outcome: "done", output: { k: env.secrets.API_KEY, big: 1n } })`, in())
	wantReason(t, res, ReasonOutputInvalid)
	check(t, res)
}
