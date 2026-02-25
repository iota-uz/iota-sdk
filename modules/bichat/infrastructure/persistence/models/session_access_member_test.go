package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionAccessModelToDomain(t *testing.T) {
	t.Parallel()

	model := &SessionAccessModel{
		Role:   "EDITOR",
		Source: "member",
	}

	access, err := model.ToDomain()
	require.NoError(t, err)
	assert.True(t, access.CanRead)
	assert.True(t, access.CanWrite)
	assert.False(t, access.CanManageMembers)
}

func TestSessionMemberModelToDomain(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	model := &SessionMemberModel{
		SessionID: uuid.New(),
		UserID:    17,
		Role:      "VIEWER",
		CreatedAt: now,
		UpdatedAt: now,
		FirstName: "John",
		LastName:  "Smith",
	}

	member, err := model.ToDomain()
	require.NoError(t, err)
	assert.Equal(t, domain.SessionMemberRoleViewer, member.Role)
	assert.Equal(t, int64(17), member.User.ID)
	assert.Equal(t, "John", member.User.FirstName)
}

func TestSessionMemberUpsertRemovalModelFromDomain(t *testing.T) {
	t.Parallel()

	upsertCmd, err := domain.NewSessionMemberUpsert(domain.SessionMemberUpsertSpec{
		SessionID: uuid.New(),
		UserID:    99,
		Role:      domain.SessionMemberRoleEditor,
	})
	require.NoError(t, err)
	upsertModel := SessionMemberUpsertModelFromDomain(upsertCmd)
	assert.Equal(t, "EDITOR", upsertModel.Role)
	assert.Equal(t, int64(99), upsertModel.UserID)

	removalCmd, err := domain.NewSessionMemberRemoval(domain.SessionMemberRemovalSpec{
		SessionID: uuid.New(),
		UserID:    99,
	})
	require.NoError(t, err)
	removalModel := SessionMemberRemovalModelFromDomain(removalCmd)
	assert.Equal(t, int64(99), removalModel.UserID)
}
