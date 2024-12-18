package user

import (
	"strings"
	"time"

	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/role"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/permission"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/upload"
	"github.com/iota-agency/iota-sdk/pkg/utils/sequence"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID         uint
	FirstName  string `validate:"required"`
	LastName   string `validate:"required"`
	MiddleName string
	Password   string
	Email      string `validate:"required,email"`
	AvatarID   *uint
	Avatar     *upload.Upload
	EmployeeID *uint
	LastIP     *string
	UILanguage UILanguage
	LastLogin  *time.Time
	LastAction *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Roles      []*role.Role
}

func (u *User) CheckPassword(password string) bool {
	if u.Password == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) == nil
}

func (u *User) Can(perm permission.Permission) bool {
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
