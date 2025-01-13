package user

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
)

type User interface {
	ID() uint
	FirstName() string
	LastName() string
	MiddleName() string
	Password() string
	Email() string
	AvatarID() uint
	Avatar() *upload.Upload
	EmployeeID() uint
	LastIP() string
	UILanguage() UILanguage
	Roles() []role.Role
	LastLogin() time.Time
	LastAction() time.Time
	CreatedAt() time.Time
	UpdatedAt() time.Time

	Can(perm *permission.Permission) bool
	CheckPassword(password string) bool

	AddRole(r role.Role) User
	SetName(firstName, lastName, middleName string) User
	SetPassword(password string) (User, error)
	SetEmail(email string) User
}
