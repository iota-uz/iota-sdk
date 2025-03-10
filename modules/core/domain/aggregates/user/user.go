package user

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/utils/sequence"

	"golang.org/x/crypto/bcrypt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
)

type Option func(u *user)

// --- Option setters ---

func WithID(id uint) Option {
	return func(u *user) {
		u.id = id
	}
}

func WithMiddleName(middleName string) Option {
	return func(u *user) {
		u.middleName = middleName
	}
}

func WithPassword(password string) Option {
	return func(u *user) {
		u.password = password
	}
}

func WithAvatar(avatar upload.Upload) Option {
	return func(u *user) {
		u.avatar = avatar
		if avatar != nil {
			u.avatarID = avatar.ID()
		}
	}
}

func WithAvatarID(id uint) Option {
	return func(u *user) {
		u.avatarID = id
	}
}

func WithRoles(roles []role.Role) Option {
	return func(u *user) {
		u.roles = roles
	}
}

func WithGroupIDs(groupIDs []uuid.UUID) Option {
	return func(u *user) {
		u.groupIDs = groupIDs
	}
}

func WithLastIP(ip string) Option {
	return func(u *user) {
		u.lastIP = ip
	}
}

func WithLastLogin(t time.Time) Option {
	return func(u *user) {
		u.lastLogin = t
	}
}

func WithLastAction(t time.Time) Option {
	return func(u *user) {
		u.lastAction = t
	}
}

func WithCreatedAt(t time.Time) Option {
	return func(u *user) {
		u.createdAt = t
	}
}

func WithUpdatedAt(t time.Time) Option {
	return func(u *user) {
		u.updatedAt = t
	}
}

// ---- Interfaces ----

type User interface {
	ID() uint
	FirstName() string
	LastName() string
	MiddleName() string
	Password() string
	Email() internet.Email
	AvatarID() uint
	Avatar() upload.Upload
	LastIP() string
	UILanguage() UILanguage
	Roles() []role.Role
	GroupIDs() []uuid.UUID
	LastLogin() time.Time
	LastAction() time.Time
	CreatedAt() time.Time
	UpdatedAt() time.Time

	Can(perm *permission.Permission) bool
	CheckPassword(password string) bool

	AddRole(r role.Role) User
	AddGroupID(groupID uuid.UUID) User
	RemoveGroupID(groupID uuid.UUID) User
	SetName(firstName, lastName, middleName string) User
	SetUILanguage(lang UILanguage) User
	SetAvatarID(id uint) User
	SetLastIP(ip string) User
	SetPassword(password string) (User, error)
	SetPasswordUnsafe(password string) User
	SetEmail(email internet.Email) User
}

// ---- Implementation ----

func New(
	firstName, lastName string,
	email internet.Email,
	uiLanguage UILanguage,
	opts ...Option,
) User {
	u := &user{
		id:         0,
		firstName:  firstName,
		lastName:   lastName,
		middleName: "",
		password:   "",
		email:      email,
		avatarID:   0,
		avatar:     nil,
		lastIP:     "",
		uiLanguage: uiLanguage,
		roles:      []role.Role{},
		groupIDs:   []uuid.UUID{},
		lastLogin:  time.Time{},
		lastAction: time.Time{},
		createdAt:  time.Now(),
		updatedAt:  time.Now(),
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

type user struct {
	id         uint
	firstName  string
	lastName   string
	middleName string
	password   string
	email      internet.Email
	avatarID   uint
	avatar     upload.Upload
	lastIP     string
	uiLanguage UILanguage
	roles      []role.Role
	groupIDs   []uuid.UUID
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

func (u *user) Email() internet.Email {
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

func (u *user) GroupIDs() []uuid.UUID {
	return u.groupIDs
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
	result := *u
	result.roles = append(result.roles, r)
	result.updatedAt = time.Now()
	return &result
}

func (u *user) AddGroupID(groupID uuid.UUID) User {
	result := *u
	result.groupIDs = append(result.groupIDs, groupID)
	result.updatedAt = time.Now()
	return &result
}

func (u *user) RemoveGroupID(groupID uuid.UUID) User {
	result := *u
	filteredGroups := make([]uuid.UUID, 0, len(result.groupIDs))
	for _, id := range result.groupIDs {
		if id == groupID {
			continue
		}
		filteredGroups = append(filteredGroups, id)
	}
	result.groupIDs = filteredGroups
	result.updatedAt = time.Now()
	return &result
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
	result := *u
	result.firstName = firstName
	result.lastName = lastName
	result.middleName = middleName
	result.updatedAt = time.Now()
	return &result
}

func (u *user) SetEmail(email internet.Email) User {
	result := *u
	result.email = email
	result.updatedAt = time.Now()
	return &result
}

func (u *user) SetUILanguage(lang UILanguage) User {
	result := *u
	result.uiLanguage = lang
	result.updatedAt = time.Now()
	return &result
}

func (u *user) SetAvatarID(id uint) User {
	result := *u
	result.avatarID = id
	result.updatedAt = time.Now()
	return &result
}

func (u *user) SetLastIP(ip string) User {
	result := *u
	result.lastIP = ip
	result.updatedAt = time.Now()
	return &result
}

func (u *user) SetPassword(password string) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	result := *u
	result.password = string(hash)
	result.updatedAt = time.Now()
	return &result, nil
}

func (u *user) SetPasswordUnsafe(newPassword string) User {
	result := *u
	result.password = newPassword
	result.updatedAt = time.Now()
	return &result
}
