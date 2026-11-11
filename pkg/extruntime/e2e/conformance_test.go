package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// conformanceDir holds the shared host conformance suite: the probe package
// and the cases web/runtime/tests/conformance.test.ts runs on the TS test host.
var conformanceDir = filepath.Join("web", "runtime", "tests", "conformance")

type confStep struct {
	Via    string          `json:"via"`
	Step   json.RawMessage `json:"step"`
	Expect struct {
		OK    json.RawMessage `json:"ok"`
		Error string          `json:"error"`
	} `json:"expect"`
}

type confCase struct {
	Name string `json:"name"`
	User struct {
		Permissions []string `json:"permissions"`
	} `json:"user"`
	Seed []struct {
		Label   string          `json:"label"`
		Entity  string          `json:"entity"`
		Overlay string          `json:"overlay"`
		Base    json.RawMessage `json:"base"`
		Values  json.RawMessage `json:"values"`
	} `json:"seed"`
	Steps []confStep `json:"steps"`
}

func loadDirTree(tb testing.TB, root string) tree {
	tb.Helper()
	out := tree{}
	require.NoError(tb, filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		b, err := os.ReadFile(p)
		out[filepath.ToSlash(rel)] = string(b)
		return err
	}))
	return out
}

// bindRefs replaces {"$ref": label} by the seeded row's ref; labelRefs is the
// inverse on results, so outputs compare without host-specific keys.
func bindRefs(v any, refs map[string]spi.ObjectRef) any {
	switch x := v.(type) {
	case []any:
		for i := range x {
			x[i] = bindRefs(x[i], refs)
		}
	case map[string]any:
		if l, ok := x["$ref"].(string); ok && len(x) == 1 {
			r, ok := refs[l]
			if !ok {
				panic("unknown seed label " + l)
			}
			return map[string]any{"entity": r.Entity, "key": r.Key}
		}
		for k := range x {
			x[k] = bindRefs(x[k], refs)
		}
	}
	return v
}

func labelRefs(v any, refs map[string]spi.ObjectRef) any {
	switch x := v.(type) {
	case []any:
		for i := range x {
			x[i] = labelRefs(x[i], refs)
		}
	case map[string]any:
		ent, ok1 := x["entity"].(string)
		key, ok2 := x["key"].(string)
		if ok1 && ok2 && len(x) == 2 {
			for l, r := range refs {
				if r.Entity == ent && r.Key == key {
					return map[string]any{"$ref": l}
				}
			}
			return map[string]any{"$ref": "?"}
		}
		for k := range x {
			x[k] = labelRefs(x[k], refs)
		}
	}
	return v
}

func mustJSON(tb testing.TB, v any) json.RawMessage {
	tb.Helper()
	b, err := json.Marshal(v)
	require.NoError(tb, err)
	return b
}

func decodeAny(tb testing.TB, raw json.RawMessage) any {
	tb.Helper()
	if len(raw) == 0 {
		return nil
	}
	var v any
	require.NoError(tb, json.Unmarshal(raw, &v))
	return v
}

// TestHostConformance runs the shared conformance cases against the Go host
// with the PostgreSQL data store and the real Node worker: the same package
// (built by the real build worker against the real kit) and the same cases as
// the TS test host run, so the two hosts cannot drift apart unnoticed.
func TestHostConformance(t *testing.T) {
	e := newEnv(t)
	const ns = "e2e.conformance"
	e.deploy(e.admin, ns, "1.0.0", loadDirTree(t, filepath.Join(conformanceDir, "package")))

	raw, err := os.ReadFile(filepath.Join(conformanceDir, "cases.json"))
	require.NoError(t, err)
	var doc struct {
		Cases []confCase `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(raw, &doc))
	require.NotEmpty(t, doc.Cases)

	ctx := context.Background()
	subject := func(perms []string) spi.Subject {
		return spi.Subject{TenantID: e.tenant, UserID: int64(e.admin.id), Kind: spi.SubjectInitiator, Permissions: perms, Locale: "en"}
	}
	root := subject([]string{ns + ".readAll", ns + ".readPublic", ns + ".seeSecret"})
	contract := map[string]string{"write": ns + ".write", "read": ns + ".read", "flow": ns + ".flow"}
	invoke := func(sub spi.Subject, via string, step any) (any, string) {
		res, err := e.rt.Host.Invoke(ctx, host.Request{
			Contract: contract[via], Input: mustJSON(t, map[string]any{"step": step}), Subject: sub, Trigger: host.Trigger{Kind: host.TriggerUI},
		})
		if err != nil {
			var we *pool.WireError
			if errors.As(err, &we) {
				return nil, we.Code
			}
			return nil, "<error>"
		}
		var out struct {
			Result any `json:"result"`
		}
		require.NoError(t, json.Unmarshal(res.Output, &out))
		return out.Result, ""
	}

	for _, k := range doc.Cases {
		t.Run(k.Name, func(t *testing.T) {
			_, err := e.pool.Exec(ctx, `DO $$ DECLARE r record; BEGIN
				FOR r IN SELECT table_name FROM information_schema.tables WHERE table_schema = 'x_e2e_conformance' LOOP
					EXECUTE format('TRUNCATE x_e2e_conformance.%I CASCADE', r.table_name);
				END LOOP; END $$`)
			require.NoError(t, err)
			refs := map[string]spi.ObjectRef{}
			for _, s := range k.Seed {
				values := bindRefs(decodeAny(t, s.Values), refs)
				if s.Overlay != "" {
					_, code := invoke(root, "write", map[string]any{"op": "put", "overlay": s.Overlay, "base": bindRefs(decodeAny(t, s.Base), refs), "values": values, "select": []string{}})
					require.Empty(t, code, "seed overlay")
					continue
				}
				out, code := invoke(root, "write", map[string]any{"op": "insert", "entity": s.Entity, "values": values, "select": []string{"ref"}})
				require.Empty(t, code, "seed %s", s.Label)
				if s.Label != "" {
					var r struct {
						Ref spi.ObjectRef `json:"ref"`
					}
					require.NoError(t, json.Unmarshal(mustJSON(t, out), &r))
					refs[s.Label] = r.Ref
				}
			}
			sub := subject(k.User.Permissions)
			for i, st := range k.Steps {
				out, code := invoke(sub, st.Via, bindRefs(decodeAny(t, st.Step), refs))
				switch {
				case st.Expect.Error == "*":
					require.NotEmpty(t, code, "step %d: %s: an error", i, st.Step)
				case st.Expect.Error != "":
					require.Equal(t, st.Expect.Error, code, "step %d: %s", i, st.Step)
				default:
					require.Empty(t, code, "step %d: %s", i, st.Step)
					require.JSONEq(t, string(st.Expect.OK), string(mustJSON(t, labelRefs(out, refs))), "step %d: %s", i, st.Step)
				}
			}
		})
	}
}
