package event

import (
	"github.com/iota-agency/iota-erp/internal/domain/entities/session"
	"github.com/iota-agency/iota-erp/internal/domain/entities/user"
)

type Event interface {
	Name() string
	Data() interface{}
	Sender() *user.User
	Session() *session.Session
}
