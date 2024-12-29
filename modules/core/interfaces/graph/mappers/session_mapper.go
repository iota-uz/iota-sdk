package mappers

import (
	model "github.com/iota-uz/iota-sdk/modules/core/interfaces/graph/gqlmodels"
	"github.com/iota-uz/iota-sdk/pkg/domain/entities/session"
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
