package project_stages

import (
	"github.com/iota-agency/iota-erp/internal/domain/entities/session"
	"github.com/iota-agency/iota-erp/internal/domain/entities/user"
)

type Created struct {
	Sender  *user.User
	Session *session.Session
	Data    *CreateDTO
	Result  *ProjectStage
}

type Updated struct {
	Sender  *user.User
	Session *session.Session
	Data    *UpdateDTO
	Result  *ProjectStage
}

type Deleted struct {
	Sender  *user.User
	Session *session.Session
	Result  *ProjectStage
}
