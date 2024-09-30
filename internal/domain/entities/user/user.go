package user

import (
	"strings"
	"time"

	"github.com/iota-agency/iota-erp/internal/domain/entities/role"
	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
	"github.com/iota-agency/iota-erp/sdk/utils/sequence"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/crypto/bcrypt"
)

type UserRole struct {
	RoleId int64
	UserId int64
	Role   role.Role
}

type User struct {
	Id         int64
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	MiddleName *string
	Password   *string
	Email      string
	AvatarID   *int64
	EmployeeID *int64
	LastIp     *string
	LastLogin  *time.Time
	LastAction *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Roles      []*role.Role `gorm:"many2many:user_roles"`
}

type UserUpdate struct {
	FirstName string
	LastName  string
	Email     string
	Password  string
	RoleId    int64
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

func (u *User) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	if strings.TrimSpace(u.FirstName) == "" {
		errors["firstName"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.Required"})
	}
	if strings.TrimSpace(u.LastName) == "" {
		errors["lastName"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.Required"})
	}
	if strings.TrimSpace(u.Email) == "" {
		errors["email"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.Required"})
	}
	if v := u.Password; v == nil || strings.TrimSpace(*v) == "" {
		errors["password"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.Required"})
	}
	return errors, len(errors) == 0
}

func (u *UserUpdate) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	if strings.TrimSpace(u.FirstName) == "" {
		errors["firstName"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.Required"})
	}
	if strings.TrimSpace(u.FirstName) == "" {
		errors["lastName"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.Required"})
	}
	if strings.TrimSpace(u.FirstName) == "" {
		errors["email"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.Required"})
	}
	if u.RoleId == 0 {
		errors["roleId"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Validations.Required"})
	}
	return errors, len(errors) == 0
}

func (u *User) ToGraph() *model.User {
	return &model.User{
		ID:         u.Id,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		MiddleName: u.MiddleName,
		Email:      u.Email,
		AvatarID:   u.AvatarID,
		EmployeeID: u.EmployeeID,
		LastIP:     u.LastIp,
		LastLogin:  u.LastLogin,
		LastAction: u.LastAction,
		CreatedAt:  u.CreatedAt,
		UpdatedAt:  u.UpdatedAt,
	}
}
