package user

import "github.com/iota-agency/iota-erp/internal/domain/session"

type Created struct {
	User    *User
	Session *session.Session
	Data    *User
	Result  *User
}

type Updated struct {
	User    *User
	Session *session.Session
	Data    *User
	Result  *User
}

type Deleted struct {
	User    *User
	Session *session.Session
	Result  *User
}
