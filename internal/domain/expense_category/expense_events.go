package category

import (
	"github.com/iota-agency/iota-erp/internal/domain/session"
	"github.com/iota-agency/iota-erp/internal/domain/user"
)

type Created struct {
	Sender  *user.User
	Session *session.Session
	Data    *ExpenseCategory
	Result  *ExpenseCategory
}

type Updated struct {
	Sender  *user.User
	Session *session.Session
	Data    *ExpenseCategory
	Result  *ExpenseCategory
}

type Deleted struct {
	Sender  *user.User
	Session *session.Session
	Result  *ExpenseCategory
}
