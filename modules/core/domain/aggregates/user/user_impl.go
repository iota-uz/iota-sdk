package user

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/utils/sequence"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
)

func New(
	firstName, lastName, middleName, password, email string,
	avatar *upload.Upload, employeeID uint,
	uiLanguage UILanguage,
	roles []role.Role,
) User {
	var avatarID uint
	if avatar != nil {
		avatarID = avatar.ID
	}
	return &user{
		id:         0,
		firstName:  firstName,
		lastName:   lastName,
		middleName: middleName,
		password:   password,
		email:      email,
		avatarID:   avatarID,
		avatar:     avatar,
		employeeID: employeeID,
		lastIP:     "",
		uiLanguage: uiLanguage,
		roles:      roles,
		lastLogin:  time.Time{},
		lastAction: time.Time{},
		createdAt:  time.Now(),
		updatedAt:  time.Now(),
	}
}

func NewWithID(
	id uint,
	firstName, lastName, middleName, password, email string,
	avatar *upload.Upload, employeeID uint,
	lastIP string,
	uiLanguage UILanguage,
	roles []role.Role,
	lastLogin, lastAction, createdAt, updatedAt time.Time,
) User {
	var avatarID uint
	if avatar != nil {
		avatarID = avatar.ID
	}
	return &user{
		id:         id,
		firstName:  firstName,
		lastName:   lastName,
		middleName: middleName,
		password:   password,
		email:      email,
		avatarID:   avatarID,
		avatar:     avatar,
		employeeID: employeeID,
		lastIP:     lastIP,
		uiLanguage: uiLanguage,
		roles:      roles,
		lastLogin:  lastLogin,
		lastAction: lastAction,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}
}

type user struct {
	id         uint
	firstName  string
	lastName   string
	middleName string
	password   string
	email      string
	avatarID   uint
	avatar     *upload.Upload
	employeeID uint
	lastIP     string
	uiLanguage UILanguage
	roles      []role.Role
	lastLogin  time.Time
	lastAction time.Time
	createdAt  time.Time
	updatedAt  time.Time
}

func (u *user) ID() uint {
	return u.id
}

func (u *user) FirstName() string {
	return u.firstName
}

func (u *user) LastName() string {
	return u.lastName
}

func (u *user) MiddleName() string {
	return u.middleName
}

func (u *user) Password() string {
	return u.password
}

func (u *user) Email() string {
	return u.email
}

func (u *user) AvatarID() uint {
	return u.avatarID
}

func (u *user) Avatar() *upload.Upload {
	return u.avatar
}

func (u *user) EmployeeID() uint {
	return u.employeeID
}

func (u *user) LastIP() string {
	return u.lastIP
}

func (u *user) UILanguage() UILanguage {
	return u.uiLanguage
}

func (u *user) Roles() []role.Role {
	return u.roles
}

func (u *user) FullName() string {
	out := new(strings.Builder)
	if u.firstName != "" {
		out.WriteString(u.firstName)
	}
	if u.middleName != "" {
		sequence.Pad(out, " ")
		out.WriteString(u.middleName)
	}
	if u.lastName != "" {
		sequence.Pad(out, " ")
		out.WriteString(u.lastName)
	}
	return out.String()
}

func (u *user) AddRole(r role.Role) User {
	return &user{
		id:         u.id,
		firstName:  u.firstName,
		lastName:   u.lastName,
		middleName: u.middleName,
		password:   u.password,
		email:      u.email,
		avatarID:   u.avatarID,
		avatar:     u.avatar,
		employeeID: u.employeeID,
		lastIP:     u.lastIP,
		uiLanguage: u.uiLanguage,
		roles:      append(u.roles, r),
		lastLogin:  u.lastLogin,
		lastAction: u.lastAction,
		createdAt:  u.createdAt,
		updatedAt:  time.Now(),
	}
}

func (u *user) LastLogin() time.Time {
	return u.lastLogin
}

func (u *user) LastAction() time.Time {
	return u.lastAction
}

func (u *user) CreatedAt() time.Time {
	return u.createdAt
}

func (u *user) UpdatedAt() time.Time {
	return u.updatedAt
}

func (u *user) Can(perm *permission.Permission) bool {
	for _, r := range u.roles {
		if r.Can(perm) {
			return true
		}
	}
	return false
}

func (u *user) CheckPassword(password string) bool {
	if u.password == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(u.password), []byte(password)) == nil
}

func (u *user) SetName(firstName, middleName, lastName string) User {
	return &user{
		id:         u.id,
		firstName:  firstName,
		lastName:   lastName,
		middleName: middleName,
		password:   u.password,
		email:      u.email,
		avatarID:   u.avatarID,
		avatar:     u.avatar,
		employeeID: u.employeeID,
		lastIP:     u.lastIP,
		uiLanguage: u.uiLanguage,
		roles:      u.roles,
		lastLogin:  u.lastLogin,
		lastAction: u.lastAction,
		createdAt:  u.createdAt,
		updatedAt:  time.Now(),
	}
}

func (u *user) SetEmail(email string) User {
	return &user{
		id:         u.id,
		firstName:  u.firstName,
		lastName:   u.lastName,
		middleName: u.middleName,
		password:   u.password,
		email:      email,
		avatarID:   u.avatarID,
		avatar:     u.avatar,
		employeeID: u.employeeID,
		lastIP:     u.lastIP,
		uiLanguage: u.uiLanguage,
		roles:      u.roles,
		lastLogin:  u.lastLogin,
		lastAction: u.lastAction,
		createdAt:  u.createdAt,
		updatedAt:  time.Now(),
	}
}

func (u *user) SetPassword(password string) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	newPassword := string(hash)
	return &user{
		id:         u.id,
		firstName:  u.firstName,
		lastName:   u.lastName,
		middleName: u.middleName,
		password:   newPassword,
		email:      u.email,
		avatarID:   u.avatarID,
		avatar:     u.avatar,
		employeeID: u.employeeID,
		lastIP:     u.lastIP,
		uiLanguage: u.uiLanguage,
		roles:      u.roles,
		lastLogin:  u.lastLogin,
		lastAction: u.lastAction,
		createdAt:  u.createdAt,
		updatedAt:  time.Now(),
	}, nil
}
