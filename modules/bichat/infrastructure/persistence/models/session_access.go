package models

import (
	"errors"

	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
)

var (
	ErrNilSessionAccessModel = errors.New("session access model is nil")
)

// SessionAccessModel is a persistence projection of resolved access.
type SessionAccessModel struct {
	Role   string
	Source string
}

// ToDomain converts the model to a domain SessionAccess value object.
func (m *SessionAccessModel) ToDomain() (domain.SessionAccess, error) {
	if m == nil {
		return domain.SessionAccess{}, ErrNilSessionAccessModel
	}
	return domain.NewSessionAccess(
		domain.ParseSessionMemberRole(m.Role),
		domain.ParseSessionAccessSource(m.Source),
	)
}
