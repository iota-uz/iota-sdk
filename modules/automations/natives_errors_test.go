package automations_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

// Every domain error a contract of the SDK's native set lists is a declared domain error of the set: the
// extension kit resolves each of them when it is built (Node-free check of what pkg/extruntime/e2e
// meets in the kit build, "references automation.forbidden, which no kit source declares").
func TestNativeContractErrorsAreDeclared(t *testing.T) {
	t.Parallel()
	set, err := native.Compose(modules.Natives()...)
	require.NoError(t, err)
	listed := 0
	for _, r := range set.Resources() {
		for _, id := range r.Errors {
			listed++
			declared, ok := set.Resource(id)
			if assert.Truef(t, ok, "%s lists the error %s, which the set does not declare", r.ID, id) {
				assert.Equalf(t, manifest.ResDomainError, declared.Kind, "%s lists %s, which is not a domain error", r.ID, id)
			}
		}
	}
	assert.Positive(t, listed, "the set lists no contract errors: the check proves nothing")
}
