package user

import (
	"strings"
	"time"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/utils/sequence"
	"golang.org/x/crypto/bcrypt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

type User struct {
	ID         uint
	FirstName  string
	LastName   string
	MiddleName string
	Password   string
	Email      string
	AvatarID   *uint
	Avatar     *upload.Upload
	EmployeeID *uint
	LastIP     *string
	UILanguage UILanguage
	Roles      []role.Role
	LastLogin  *time.Time
	LastAction *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (u *User) SetName(firstName, middleName, lastName string) *User {
	return &User{
		ID:         u.ID,
		FirstName:  firstName,
		LastName:   lastName,
		MiddleName: middleName,
		Password:   u.Password,
		Email:      u.Email,
		AvatarID:   u.AvatarID,
		Avatar:     u.Avatar,
		EmployeeID: u.EmployeeID,
		LastIP:     u.LastIP,
		UILanguage: u.UILanguage,
		Roles:      u.Roles,
		LastLogin:  u.LastLogin,
		LastAction: u.LastAction,
		CreatedAt:  u.CreatedAt,
		UpdatedAt:  time.Now(),
	}
}

func (u *User) CheckPassword(password string) bool {
	if u.Password == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) == nil
}

func (u *User) Can(perm *permission.Permission) bool {
	for _, r := range u.Roles {
		if r.Can(perm) {
			return true
		}
	}
	return false
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	newPassword := string(hash)
	u.Password = newPassword
	return nil
}

func (u *User) FullName() string {
	out := new(strings.Builder)
	if u.FirstName != "" {
		out.WriteString(u.FirstName)
	}
	if u.MiddleName != "" {
		sequence.Pad(out, " ")
		out.WriteString(u.MiddleName)
	}
	if u.LastName != "" {
		sequence.Pad(out, " ")
		out.WriteString(u.LastName)
	}
	return out.String()
}

func (u *User) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := constants.Validate.Struct(u)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}
