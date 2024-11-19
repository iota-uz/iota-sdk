package authlog

import (
	"time"

	model "github.com/iota-agency/iota-sdk/internal/interfaces/graph/gqlmodels"
)

type AuthenticationLog struct {
	ID        int64
	UserID    uint
	IP        string
	UserAgent string
	CreatedAt time.Time
}

func (r *AuthenticationLog) ToGraph() *model.AuthenticationLog {
	return &model.AuthenticationLog{
		ID:        r.ID,
		UserID:    int64(r.UserID),
		IP:        r.IP,
		UserAgent: r.UserAgent,
		CreatedAt: r.CreatedAt,
	}
}
