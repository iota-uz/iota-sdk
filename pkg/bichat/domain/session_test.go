package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSession_ValidatesSpec(t *testing.T) {
	t.Parallel()

	_, err := domain.NewSession(domain.SessionSpec{})
	require.Error(t, err)

	s, err := domain.NewSession(domain.SessionSpec{
		TenantID:    uuid.New(),
		OwnerUserID: 42,
		Title:       "  Revenue Analysis  ",
	})
	require.NoError(t, err)
	assert.Equal(t, "Revenue Analysis", s.Title())
	assert.Equal(t, domain.SessionStatusActive, s.Status())
	assert.False(t, s.Pinned())
}

func TestNewUntitledSession(t *testing.T) {
	t.Parallel()

	s, err := domain.NewUntitledSession(domain.SessionSpec{
		TenantID:    uuid.New(),
		OwnerUserID: 100,
	})
	require.NoError(t, err)
	assert.Equal(t, "Untitled Session", s.Title())
}

func TestRehydrateSession_AllowsArchived(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	s, err := domain.RehydrateSession(domain.SessionState{
		ID:          uuid.New(),
		TenantID:    uuid.New(),
		OwnerUserID: 9,
		Title:       "Persisted",
		Status:      domain.SessionStatusArchived,
		Pinned:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	require.NoError(t, err)
	assert.True(t, s.IsArchived())
	assert.True(t, s.Pinned())
}

func TestSessionTransitions_EnforceRules(t *testing.T) {
	t.Parallel()

	created, err := domain.NewSession(domain.SessionSpec{
		TenantID:    uuid.New(),
		OwnerUserID: 5,
		Title:       "Ops",
	})
	require.NoError(t, err)

	pinned, err := created.Pin(time.Now())
	require.NoError(t, err)
	assert.True(t, pinned.Pinned())

	archived, err := pinned.Archive(time.Now())
	require.NoError(t, err)
	assert.True(t, archived.IsArchived())

	_, err = archived.Pin(time.Now())
	require.Error(t, err)

	unarchived, err := archived.Unarchive(time.Now())
	require.NoError(t, err)
	assert.True(t, unarchived.IsActive())

	_, err = unarchived.Unarchive(time.Now())
	require.Error(t, err)
}

func TestSessionRename_RejectsInvalidTitle(t *testing.T) {
	t.Parallel()

	s, err := domain.NewSession(domain.SessionSpec{
		TenantID:    uuid.New(),
		OwnerUserID: 1,
		Title:       "Name",
	})
	require.NoError(t, err)

	_, err = s.Rename("   ", time.Now())
	require.Error(t, err)

	renamed, err := s.Rename("  New Name  ", time.Now())
	require.NoError(t, err)
	assert.Equal(t, "New Name", renamed.Title())
}
