package services_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/token"
	"github.com/iota-uz/iota-sdk/modules/oidc/services"
)

type recordingTokenRepository struct {
	token.Repository
	deletedUserIDs []int
}

func (r *recordingTokenRepository) DeleteByUserID(_ context.Context, userID int) error {
	r.deletedUserIDs = append(r.deletedUserIDs, userID)
	return nil
}

func TestTokenRevocationHandler_RevokesOnPasswordChange(t *testing.T) {
	t.Parallel()
	repo := &recordingTokenRepository{}

	services.NewTokenRevocationHandler(nil, repo, nil).OnPasswordUpdated(&user.UpdatedPasswordEvent{UserID: 42})

	assert.Equal(t, []int{42}, repo.deletedUserIDs)
}
