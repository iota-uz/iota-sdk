package code_test

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/code"
)

func TestValidateParamsReportsCompileErrors(t *testing.T) {
	n := code.New(code.Deps{})
	errs := n.ValidateParams(context.Background(), map[string]any{"lang": "ts", "source": "export default async function run( {\n  return 1;\n}"})
	if len(errs) != 1 || errs[0].Path != "params.source" || errs[0].Line == 0 {
		t.Fatalf("errs: %+v", errs)
	}
}
