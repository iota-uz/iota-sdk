package mappers

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	model "github.com/iota-uz/iota-sdk/modules/core/interfaces/graph/gqlmodels"
)

func SessionToGraphModel(s *user.Session) *model.Session {
	return &model.Session{
		Token:     string(s.Token),
		IP:        s.IP,
		UserAgent: s.UserAgent,
		UserID:    int64(s.UserID),
		ExpiresAt: s.ExpiresAt,
		CreatedAt: s.CreatedAt,
	}
}
