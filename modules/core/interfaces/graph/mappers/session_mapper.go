package mappers

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	model "github.com/iota-uz/iota-sdk/modules/core/interfaces/graph/gqlmodels"
)

func SessionToGraphModel(s *session.Session) *model.Session {
	return &model.Session{
		Token:     s.Token,
		IP:        s.IP,
		UserAgent: s.UserAgent,
		UserID:    int64(s.UserID),
		ExpiresAt: s.ExpiresAt,
		CreatedAt: s.CreatedAt,
	}
}
