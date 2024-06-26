package user

import "github.com/iota-agency/iota-erp/internal/domain/session"

type Created struct {
	Sender  *User
	Session *session.Session
	Data    *User
	Result  *User
}

type Updated struct {
	Sender  *User
	Session *session.Session
	Data    *User
	Result  *User
}

type Deleted struct {
	Sender  *User
	Session *session.Session
	Result  *User
}
