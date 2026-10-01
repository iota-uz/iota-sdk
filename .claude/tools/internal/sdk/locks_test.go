package sdk

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// False green: an unchecked fake Go runner accepts a GitHub repository slug
// even though it cannot select the SDK in the consumer's Go module graph.
func TestVerifyLocks_SelectsSDKGoModule(t *testing.T) {
	ctx := context.Background()
	moduleName, err := (ExecRunner{}).Run(ctx, "../../../..", nil, "env", "GOWORK=off", "go", "list", "-m")
	require.NoError(t, err)
	runner := fakeRunner{call: func(_ string, _ []byte, name string, args []string) ([]byte, error) {
		require.Equal(t, "env", name)
		require.Equal(t, []string{"GOWORK=off", "go", "list", "-m", "-json", strings.TrimSpace(string(moduleName))}, args)
		return []byte(`{"Version":"v0.6.0"}`), nil
	}}
	require.NoError(t, VerifyLocks(ctx, runner, t.TempDir(), Dependency{GoDir: ".", Version: "0.6.0", SHA: strings.Repeat("a", 40)}))
}
