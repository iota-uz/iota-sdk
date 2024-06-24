package event

import (
	"github.com/iota-agency/iota-erp/internal/domain/session"
	"github.com/iota-agency/iota-erp/internal/domain/user"
)

type Event interface {
	Name() string
	Data() interface{}
	Sender() *user.User
	Session() *session.Session
}
