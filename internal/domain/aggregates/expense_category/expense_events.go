package category

import (
	"github.com/iota-agency/iota-erp/internal/domain/entities/session"
	"github.com/iota-agency/iota-erp/internal/domain/entities/user"
)

type Created struct {
	Sender  *user.User
	Session *session.Session
	Data    *CreateDTO
	Result  *ExpenseCategory
}

type Updated struct {
	Sender  *user.User
	Session *session.Session
	Data    *UpdateDTO
	Result  *ExpenseCategory
}

type Deleted struct {
	Sender  *user.User
	Session *session.Session
	Result  *ExpenseCategory
}
