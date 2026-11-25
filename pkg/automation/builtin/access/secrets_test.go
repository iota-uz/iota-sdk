package access_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/access"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

type tenantSecrets struct {
	values map[string]string
	err    error
	reads  *int
}

func (r tenantSecrets) Has(_ context.Context, _ uuid.UUID, name string) (bool, error) {
	if r.err != nil {
		return false, r.err
	}
	_, ok := r.values[name]
	return ok, nil
}

func (r tenantSecrets) Resolve(_ context.Context, _ uuid.UUID, name string) (string, error) {
	if r.reads != nil {
		*r.reads++
	}
	if r.err != nil {
		return "", r.err
	}
	if v, ok := r.values[name]; ok {
		return v, nil
	}
	return "", secrets.ErrNotFound
}

type countingSecrets struct {
	values map[string]string
	calls  int
}

func (c *countingSecrets) Secret(_ context.Context, _ uuid.UUID, _ spi.Subject, name string) (string, error) {
	c.calls++
	if v, ok := c.values[name]; ok {
		return v, nil
	}
	return "", access.ErrSecretMissing
}

const useSecrets = "Secrets.Use"

var user = spi.Subject{Permissions: []string{useSecrets}}

func TestChainTenantSecretFirstThenTheApplication(t *testing.T) {
	app := &countingSecrets{values: map[string]string{"API_KEY": "from-app", "ONLY_APP": "app"}}
	s := access.ChainSecrets(tenantSecrets{values: map[string]string{"API_KEY": "from-tenant"}}, useSecrets, app)
	ctx := context.Background()

	v, err := s.Secret(ctx, uuid.New(), user, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "from-tenant", v)
	assert.Zero(t, app.calls)

	v, err = s.Secret(ctx, uuid.New(), user, "ONLY_APP")
	require.NoError(t, err)
	assert.Equal(t, "app", v)

	_, err = s.Secret(ctx, uuid.New(), user, "NOPE")
	assert.ErrorIs(t, err, access.ErrSecretMissing)
}

func TestChainRefusesARunAsWithoutSecretsUse(t *testing.T) {
	app := &countingSecrets{values: map[string]string{"API_KEY": "from-app"}}
	reads := 0
	s := access.ChainSecrets(tenantSecrets{values: map[string]string{"API_KEY": "from-tenant"}, reads: &reads}, useSecrets, app)
	_, err := s.Secret(context.Background(), uuid.New(), spi.Subject{}, "API_KEY")
	require.ErrorIs(t, err, access.ErrSecretForbidden)
	assert.Zero(t, app.calls, "no fallback past a refusal")
	assert.Zero(t, reads, "the value is neither read nor its use recorded before the run-as is authorized")

	_, err = access.ReadSecrets(context.Background(), s, catalog.CallInput{TenantID: uuid.New()}, []string{"API_KEY"})
	var ce *catalog.CallError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, access.CodeSecretForbidden, ce.Code)

	// A name the tenant does not manage is the application's, whoever runs.
	app.values["OTHER"] = "x"
	v, err := s.Secret(context.Background(), uuid.New(), spi.Subject{}, "OTHER")
	require.NoError(t, err)
	assert.Equal(t, "x", v)
}

func TestChainOutageIsControlledAndNeverFallsBack(t *testing.T) {
	app := &countingSecrets{values: map[string]string{"API_KEY": "from-app"}}
	s := access.ChainSecrets(tenantSecrets{err: secrets.ErrUnavailable}, useSecrets, app)
	_, err := access.ReadSecrets(context.Background(), s, catalog.CallInput{TenantID: uuid.New(), RunAs: user}, []string{"API_KEY"})
	var ce *catalog.CallError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, access.CodeUnavailable, ce.Code)
	assert.True(t, ce.Retryable)
	assert.Zero(t, app.calls)
}

func TestReadSecretsSaysUnknownSecret(t *testing.T) {
	s := access.ChainSecrets(tenantSecrets{}, useSecrets, access.FromResolver(nil))
	_, err := access.ReadSecrets(context.Background(), s, catalog.CallInput{TenantID: uuid.New(), RunAs: user}, []string{"NOPE"})
	var ce *catalog.CallError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, access.CodeSecretMissing, ce.Code)
	assert.Equal(t, "unknown secret NOPE", ce.Message)
}
