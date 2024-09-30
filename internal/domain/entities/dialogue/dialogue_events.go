package dialogue

import (
	"github.com/iota-agency/iota-erp/internal/domain/entities/session"
	"github.com/iota-agency/iota-erp/internal/domain/entities/user"
)

type Created struct {
	Data    *Dialogue
	Result  *Dialogue
	Sender  *user.User
	Session *session.Session
}

type Updated struct {
	Data    *Dialogue
	Result  *Dialogue
	Sender  *user.User
	Session *session.Session
}

type Deleted struct {
	Result  *Dialogue
	Sender  *user.User
	Session *session.Session
}
