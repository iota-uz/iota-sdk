package code_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/access"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	sandbox "github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

type fixedTenantSecrets map[string]string

func (f fixedTenantSecrets) Resolve(_ context.Context, _ uuid.UUID, name string) (string, error) {
	if v, ok := f[name]; ok {
		return v, nil
	}
	return "", secrets.ErrNotFound
}

// A tenant secret reaches the code through the chain, and a run log it writes is masked.
func TestCodeLogsATenantSecretMasked(t *testing.T) {
	engOnce.Do(func() { eng, errEng = sandbox.NewEngine(context.Background()) })
	if errEng != nil {
		t.Fatal(errEng)
	}
	const value = "sk-live-7f3a9c1d"
	chain := access.ChainSecrets(fixedTenantSecrets{"API_KEY": value}, "Secrets.Use", access.FromResolver(nil))
	n := code.New(code.Deps{Engine: eng, Secrets: chain})
	res, err := n.Execute(context.Background(), catalog.CallInput{
		TenantID: uuid.New(), RunAs: spi.Subject{Permissions: []string{"Secrets.Use"}}, Deadline: time.Now().Add(time.Minute),
		Params: map[string]any{"lang": "js", "secrets": []any{"API_KEY"}, "source": `export default async (ctx, env) => { env.log("key", env.secrets.API_KEY); return { outcome: "done", output: { n: env.secrets.API_KEY.length } }; }`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "done" {
		t.Fatalf("%+v", res)
	}
	logs, _ := res.Output["logs"].([]string)
	joined := strings.Join(logs, "\n")
	if strings.Contains(joined, value) || !strings.Contains(joined, "***") {
		t.Fatalf("logs: %q", joined)
	}
}
