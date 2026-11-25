package automations

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

// The run view's «Open <subject>» and a run context's subject.url come from the module's link registry:
// core.user links to its page under the configured origin; a module's own entity is added by an option.
func TestSubjectLinksUnderTheOrigin(t *testing.T) {
	t.Parallel()
	c := &component{}
	WithSubjectLink("crm.client", func(_ context.Context, _ uuid.UUID, key string) (string, error) {
		return "/crm/clients/" + key, nil
	})(c)
	links, err := c.newLinks("https://erp.test")
	require.NoError(t, err)
	tenant := uuid.New()
	user, err := links.SubjectURL(context.Background(), tenant, engine.SubjectRef{Entity: "core.user", Key: "42"})
	require.NoError(t, err)
	assert.Equal(t, "https://erp.test/users/42", user)
	client, err := links.SubjectURL(context.Background(), tenant, engine.SubjectRef{Entity: "crm.client", Key: "c-7"})
	require.NoError(t, err)
	assert.Equal(t, "https://erp.test/crm/clients/c-7", client)
	none, err := links.SubjectURL(context.Background(), tenant, engine.SubjectRef{Entity: "other", Key: "1"})
	require.NoError(t, err)
	assert.Empty(t, none, "an entity without a link has none")
	a, r := uuid.New(), uuid.New()
	assert.Equal(t, "https://erp.test/automations/"+a.String()+"/runs/"+r.String(), links.RunURL(tenant, a, r))
}
