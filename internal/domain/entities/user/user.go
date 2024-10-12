package user

import (
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/role"
	"strings"
	"time"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
	"github.com/iota-agency/iota-erp/pkg/constants"
	"github.com/iota-agency/iota-erp/sdk/utils/sequence"
	"golang.org/x/crypto/bcrypt"
)

type UserRole struct {
	RoleID int64
	UserID int64
	Role   role.Role
}

type User struct {
	ID         int64
	FirstName  string `validate:"required"`
	LastName   string `validate:"required"`
	MiddleName *string
	Password   *string
	Email      string `validate:"required,email"`
	AvatarID   *int64
	EmployeeID *int64
	LastIP     *string
	LastLogin  *time.Time
	LastAction *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Roles      []*role.Role `gorm:"many2many:user_roles"`
}

type UpdateDTO struct {
	FirstName string
	LastName  string
	Email     string
	Password  string
	RoleID    uint
}

func (u *User) CheckPassword(password string) bool {
	if u.Password == nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(*u.Password), []byte(password)) == nil
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	newPassword := string(hash)
	u.Password = &newPassword
	return nil
}

func (u *User) FullName() string {
	out := new(strings.Builder)
	if u.FirstName != "" {
		out.WriteString(u.FirstName)
	}
	if v := u.MiddleName; v != nil && *v != "" {
		sequence.Pad(out, " ")
		out.WriteString(*v)
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

func (u *UpdateDTO) Ok(l ut.Translator) (map[string]string, bool) {
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

func (u *User) ToGraph() *model.User {
	return &model.User{
		ID:         u.ID,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		MiddleName: u.MiddleName,
		Email:      u.Email,
		AvatarID:   u.AvatarID,
		EmployeeID: u.EmployeeID,
		LastIP:     u.LastIP,
		LastLogin:  u.LastLogin,
		LastAction: u.LastAction,
		CreatedAt:  u.CreatedAt,
		UpdatedAt:  u.UpdatedAt,
	}
}
