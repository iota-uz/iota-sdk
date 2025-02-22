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

// ---- Interfaces ----

type User interface {
	ID() uint
	FirstName() string
	LastName() string
	MiddleName() string
	Password() string
	Email() string
	AvatarID() uint
	Avatar() upload.Upload
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
	SetUILanguage(lang UILanguage) User
	SetAvatarID(id uint) User
	SetLastIP(ip string) User
	SetPassword(password string) (User, error)
	SetEmail(email string) User
}

// ---- Implementation ----

func New(
	firstName, lastName, middleName, password, email string,
	avatar upload.Upload, uiLanguage UILanguage,
	roles []role.Role,
) User {
	var avatarID uint
	if avatar != nil {
		avatarID = avatar.ID()
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
	avatar upload.Upload, lastIP string,
	uiLanguage UILanguage,
	roles []role.Role,
	lastLogin, lastAction, createdAt, updatedAt time.Time,
) User {
	var avatarID uint
	if avatar != nil {
		avatarID = avatar.ID()
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
	avatar     upload.Upload
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

func (u *user) Avatar() upload.Upload {
	return u.avatar
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

func (u *user) SetName(firstName, lastName, middleName string) User {
	return &user{
		id:         u.id,
		firstName:  firstName,
		lastName:   lastName,
		middleName: middleName,
		password:   u.password,
		email:      u.email,
		avatarID:   u.avatarID,
		avatar:     u.avatar,
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
		lastIP:     u.lastIP,
		uiLanguage: u.uiLanguage,
		roles:      u.roles,
		lastLogin:  u.lastLogin,
		lastAction: u.lastAction,
		createdAt:  u.createdAt,
		updatedAt:  time.Now(),
	}
}

func (u *user) SetUILanguage(lang UILanguage) User {
	return &user{
		id:         u.id,
		firstName:  u.firstName,
		lastName:   u.lastName,
		middleName: u.middleName,
		password:   u.password,
		email:      u.email,
		avatarID:   u.avatarID,
		avatar:     u.avatar,
		lastIP:     u.lastIP,
		uiLanguage: lang,
		roles:      u.roles,
		lastLogin:  u.lastLogin,
		lastAction: u.lastAction,
		createdAt:  u.createdAt,
		updatedAt:  time.Now(),
	}
}

func (u *user) SetAvatarID(id uint) User {
	return &user{
		id:         u.id,
		firstName:  u.firstName,
		lastName:   u.lastName,
		middleName: u.middleName,
		password:   u.password,
		email:      u.email,
		avatarID:   id,
		avatar:     u.avatar,
		lastIP:     u.lastIP,
		uiLanguage: u.uiLanguage,
		roles:      u.roles,
		lastLogin:  u.lastLogin,
		lastAction: u.lastAction,
		createdAt:  u.createdAt,
		updatedAt:  time.Now(),
	}
}

func (u *user) SetLastIP(ip string) User {
	return &user{
		id:         u.id,
		firstName:  u.firstName,
		lastName:   u.lastName,
		middleName: u.middleName,
		password:   u.password,
		email:      u.email,
		avatarID:   u.avatarID,
		avatar:     u.avatar,
		lastIP:     ip,
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
		lastIP:     u.lastIP,
		uiLanguage: u.uiLanguage,
		roles:      u.roles,
		lastLogin:  u.lastLogin,
		lastAction: u.lastAction,
		createdAt:  u.createdAt,
		updatedAt:  time.Now(),
	}, nil
}
