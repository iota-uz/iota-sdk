package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionAccessRoleSourceMatrix(t *testing.T) {
	t.Parallel()

	_, err := domain.NewSessionAccess(domain.SessionMemberRoleOwner, domain.SessionAccessSourceOwner)
	require.NoError(t, err)

	_, err = domain.NewSessionAccess(domain.SessionMemberRoleEditor, domain.SessionAccessSourceMember)
	require.NoError(t, err)

	_, err = domain.NewSessionAccess(domain.SessionMemberRoleViewer, domain.SessionAccessSourceMember)
	require.NoError(t, err)

	_, err = domain.NewSessionAccess(domain.SessionMemberRoleReadAll, domain.SessionAccessSourcePermission)
	require.NoError(t, err)

	_, err = domain.NewSessionAccess(domain.SessionMemberRoleNone, domain.SessionAccessSourceNone)
	require.NoError(t, err)

	_, err = domain.NewSessionAccess(domain.SessionMemberRoleOwner, domain.SessionAccessSourceMember)
	require.Error(t, err)
}

func TestSessionAccessRequireAndGrantReadAll(t *testing.T) {
	t.Parallel()

	none, err := domain.NewSessionAccess(domain.SessionMemberRoleNone, domain.SessionAccessSourceNone)
	require.NoError(t, err)
	require.Error(t, none.Require(false, false))

	readAll, err := none.GrantReadAll()
	require.NoError(t, err)
	require.NoError(t, readAll.Require(false, false))
	require.Error(t, readAll.Require(true, false))
}

func TestSessionMemberLifecycle(t *testing.T) {
	t.Parallel()

	member, err := domain.NewSessionMember(domain.SessionMemberSpec{
		SessionID: uuid.New(),
		User: domain.SessionUser{
			ID:        10,
			FirstName: "Alice",
			LastName:  "Doe",
		},
		Role: domain.SessionMemberRoleEditor,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.SessionMemberRoleEditor, member.Role)

	updated, err := member.ChangeRole(domain.SessionMemberRoleViewer, time.Now())
	require.NoError(t, err)
	assert.Equal(t, domain.SessionMemberRoleViewer, updated.Role)

	_, err = member.ChangeRole(domain.SessionMemberRoleOwner, time.Now())
	require.Error(t, err)
}

func TestSessionMemberCommandsValidate(t *testing.T) {
	t.Parallel()

	role, err := domain.NewSessionMemberRole("editor")
	require.NoError(t, err)
	assert.Equal(t, domain.SessionMemberRoleEditor, role)

	_, err = domain.NewSessionMemberRole("owner")
	require.Error(t, err)

	upsert, err := domain.NewSessionMemberUpsert(domain.SessionMemberUpsertSpec{
		SessionID: uuid.New(),
		UserID:    5,
		Role:      domain.SessionMemberRoleViewer,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(5), upsert.UserID())

	_, err = domain.NewSessionMemberUpsert(domain.SessionMemberUpsertSpec{
		SessionID: uuid.New(),
		UserID:    0,
		Role:      domain.SessionMemberRoleViewer,
	})
	require.Error(t, err)

	removal, err := domain.NewSessionMemberRemoval(domain.SessionMemberRemovalSpec{
		SessionID: uuid.New(),
		UserID:    5,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(5), removal.UserID())
}
