package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/shell/worker/internal/protocol"
)

func run(t *testing.T, req protocol.Request, budgets Budgets) *protocol.Result {
	t.Helper()
	if req.Cwd == "" {
		req.Cwd = protocol.TempDir
	}
	if req.TimeoutMS == 0 {
		req.TimeoutMS = 5000
	}
	if req.MaxOutputBytes == 0 {
		req.MaxOutputBytes = 32 << 10
	}
	if verr := req.Validate(); verr != nil {
		t.Fatalf("invalid request: %v", verr)
	}
	res, err := Run(context.Background(), &req, budgets)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res
}

func limitOf(res *protocol.Result) protocol.LimitHit {
	if res.LimitHit == nil {
		return ""
	}
	return *res.LimitHit
}

func exitOf(res *protocol.Result) int {
	if res.ExitCode == nil {
		return -1
	}
	return *res.ExitCode
}

func TestRun_Pipelines(t *testing.T) {
	csv := []byte("id,name,amount\n1,alpha,10\n2,beta,20\n3,alpha,5\n")
	res := run(t, protocol.Request{
		Script: `cd /input/data
awk -F, 'NR>1{s[$2]+=$3} END{for(k in s) print k","s[k]}' sales.csv | sort > /output/totals.csv
printf '{"n":[1,2,3]}' | jq -c '.n|add'
grep -c alpha sales.csv | tee /tmp/count
for f in $(find /input -type f); do wc -l < "$f"; done
f() { cut -d, -f2 sales.csv | sed 1d | sort | uniq -c | head -n 1 | tail -n 1; }; f
diff <(echo a) <(echo a) && echo same`,
		Files: []protocol.InputFile{{Path: "data/sales.csv", Content: csv}},
	}, DefaultBudgets)

	if exitOf(res) != 0 || res.LimitHit != nil {
		t.Fatalf("exit=%d limit=%q stderr=%q", exitOf(res), limitOf(res), res.Stderr)
	}
	if want := "6\n2\n4\n      2 alpha\nsame\n"; res.Stdout != want {
		t.Fatalf("stdout = %q, want %q", res.Stdout, want)
	}
	if len(res.Outputs) != 1 || res.Outputs[0].Path != "/output/totals.csv" || string(res.Outputs[0].Content) != "alpha,15\nbeta,20\n" {
		t.Fatalf("outputs = %+v", res.Outputs)
	}
}

func TestRun_Isolation(t *testing.T) {
	tests := []struct {
		name       string
		script     string
		wantExit   int
		wantStderr string
	}{
		{"curl is not registered", "curl https://example.com", 127, "curl: command not found"},
		{"wget is not registered", "wget https://example.com", 127, "wget: command not found"},
		{"host binaries are unknown", "python3 -c 1", 127, "python3: command not found"},
		{"package managers are unknown", "apt-get install x", 127, "apt-get: command not found"},
		{"identity commands are absent", "uname -a", 127, "uname: command not found"},
		{"links are absent", "ln -s /input/a /tmp/a", 127, "ln: command not found"},
		{"awk system stays in the registry", `awk 'BEGIN{exit system("curl x")}'`, 127, "curl: command not found"},
		{"nested shell stays in the registry", `bash -c "nc -l 80"`, 127, "nc: command not found"},
		{"input is read-only", "echo x > /input/a", 126, `write "/input/a" denied`},
		{"input cannot be removed", "rm /input/a", 1, "denied"},
		{"root is not writable", "echo x > /etc/passwd", 126, `write "/etc/passwd" denied`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := run(t, protocol.Request{
				Script: tt.script,
				Files:  []protocol.InputFile{{Path: "a", Content: []byte("secret\n")}},
			}, DefaultBudgets)
			if exitOf(res) != tt.wantExit || !strings.Contains(res.Stderr, tt.wantStderr) {
				t.Fatalf("exit=%d stderr=%q, want exit=%d stderr~%q", exitOf(res), res.Stderr, tt.wantExit, tt.wantStderr)
			}
		})
	}
}

func TestRun_Limits(t *testing.T) {
	small := DefaultBudgets
	small.Commands = 50
	small.ScratchBytes = 1 << 20

	tests := []struct {
		name      string
		req       protocol.Request
		budgets   Budgets
		wantLimit protocol.LimitHit
		check     func(t *testing.T, res *protocol.Result)
	}{
		{
			name:      "large stdout with exit 0 is truncated",
			req:       protocol.Request{Script: "yes | head -c 100000", MaxOutputBytes: 1000},
			budgets:   DefaultBudgets,
			wantLimit: protocol.LimitOutput,
			check: func(t *testing.T, res *protocol.Result) {
				t.Helper()
				if exitOf(res) != 0 || !res.StdoutTruncated || len(res.Stdout) != 1000 {
					t.Fatalf("exit=%d truncated=%v len=%d", exitOf(res), res.StdoutTruncated, len(res.Stdout))
				}
			},
		},
		{
			name:      "stdout and stderr share max_output_bytes",
			req:       protocol.Request{Script: "head -c 800 /dev/zero | tr '\\0' o; head -c 800 /dev/zero | tr '\\0' e >&2", MaxOutputBytes: 1000},
			budgets:   DefaultBudgets,
			wantLimit: protocol.LimitOutput,
			check: func(t *testing.T, res *protocol.Result) {
				t.Helper()
				if len(res.Stdout)+len(res.Stderr) != 1000 || res.StdoutTruncated || !res.StderrTruncated {
					t.Fatalf("stdout=%d stderr=%d truncated=%v/%v", len(res.Stdout), len(res.Stderr), res.StdoutTruncated, res.StderrTruncated)
				}
			},
		},
		{
			name:      "output files are limited independently of text output",
			req:       protocol.Request{Script: "seq 10000 > /output/seq.txt; seq 10000", MaxOutputBytes: 100},
			budgets:   DefaultBudgets,
			wantLimit: protocol.LimitOutput,
			check: func(t *testing.T, res *protocol.Result) {
				t.Helper()
				if len(res.Outputs) != 1 || len(res.Outputs[0].Content) != 48894 || len(res.Stdout) != 100 {
					t.Fatalf("outputs=%d stdout=%d", len(res.Outputs), len(res.Stdout))
				}
			},
		},
		{
			name:      "filesystem quota covers writes",
			req:       protocol.Request{Script: "yes | head -c 2000000 > /tmp/a"},
			budgets:   small,
			wantLimit: protocol.LimitFilesystem,
		},
		{
			name:      "filesystem quota covers sparse truncate",
			req:       protocol.Request{Script: "truncate -s 100M /output/big"},
			budgets:   small,
			wantLimit: protocol.LimitFilesystem,
		},
		{
			name:      "filesystem quota covers copies",
			req:       protocol.Request{Script: "head -c 600000 /dev/zero > /tmp/a; cp /tmp/a /tmp/b"},
			budgets:   small,
			wantLimit: protocol.LimitFilesystem,
		},
		{
			name:      "removed files release quota",
			req:       protocol.Request{Script: "for i in 1 2 3 4; do head -c 600000 /dev/zero > /tmp/a; rm /tmp/a; done; echo ok"},
			budgets:   small,
			wantLimit: "",
		},
		{
			name:      "nested shells share the command budget",
			req:       protocol.Request{Script: `for i in $(seq 30); do bash -c "true; true; true"; done`},
			budgets:   small,
			wantLimit: protocol.LimitExecution,
		},
		{
			name:      "awk system shares the command budget",
			req:       protocol.Request{Script: `awk 'BEGIN{for(i=0;i<100;i++) system("true")}'`},
			budgets:   small,
			wantLimit: protocol.LimitExecution,
		},
		{
			name:      "loop budget",
			req:       protocol.Request{Script: "while :; do x=1; done"},
			budgets:   DefaultBudgets,
			wantLimit: protocol.LimitExecution,
		},
		{
			name:      "cooperative timeout has no exit code",
			req:       protocol.Request{Script: "echo started; yes > /dev/null", TimeoutMS: 200},
			budgets:   DefaultBudgets,
			wantLimit: protocol.LimitTimeout,
			check: func(t *testing.T, res *protocol.Result) {
				t.Helper()
				if res.ExitCode != nil {
					t.Fatalf("exit_code = %d, want null", *res.ExitCode)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := run(t, tt.req, tt.budgets)
			if limitOf(res) != tt.wantLimit {
				t.Fatalf("limit_hit = %q, want %q (exit=%d stderr=%q)", limitOf(res), tt.wantLimit, exitOf(res), res.Stderr)
			}
			if tt.check != nil {
				tt.check(t, res)
			}
		})
	}
}

func TestRun_FreshFilesystemPerRun(t *testing.T) {
	first := run(t, protocol.Request{Script: "echo secret > /tmp/s; echo secret > /output/s"}, DefaultBudgets)
	if exitOf(first) != 0 {
		t.Fatalf("first run: %q", first.Stderr)
	}
	second := run(t, protocol.Request{Script: "ls -A /tmp /output /input"}, DefaultBudgets)
	if strings.Contains(second.Stdout, "s\n") || len(second.Outputs) != 0 {
		t.Fatalf("second run sees previous files: %q", second.Stdout)
	}
}

func TestClip(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		n         int
		want      string
		truncated bool
	}{
		{"fits", "abc", 3, "abc", false},
		{"cut", "abcdef", 4, "abcd", true},
		{"utf8 boundary", "aжb", 2, "a", true},
		{"negative budget", "abc", -1, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, truncated := clip(tt.in, tt.n)
			if got != tt.want || truncated != tt.truncated {
				t.Fatalf("clip(%q, %d) = %q, %v", tt.in, tt.n, got, truncated)
			}
		})
	}
}

func TestRun_SymlinksDoNotEscape(t *testing.T) {
	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	for _, name := range []string{"tmp/link", "output/link"} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Linkname: "../input/secret", Typeflag: tar.TypeSymlink, Mode: 0o777}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		script     string
		wantStderr string
	}{
		{"write through symlink", "tar -xf /input/evil.tar -C /; echo pwned > /tmp/link", "symlink traversal denied"},
		{"read through symlink", "tar -xf /input/evil.tar -C /; cat /tmp/link", "symlink traversal denied"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := run(t, protocol.Request{
				Script: tt.script,
				Files: []protocol.InputFile{
					{Path: "evil.tar", Content: archive.Bytes()},
					{Path: "secret", Content: []byte("secret\n")},
				},
			}, DefaultBudgets)
			if exitOf(res) == 0 || !strings.Contains(res.Stderr, tt.wantStderr) || strings.Contains(res.Stdout, "secret") {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exitOf(res), res.Stdout, res.Stderr)
			}
			if len(res.Outputs) != 0 {
				t.Fatalf("symlink published as output: %+v", res.Outputs)
			}
		})
	}
}
