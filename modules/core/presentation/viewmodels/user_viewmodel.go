package viewmodels

import (
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type User struct {
	ID          string
	Type        string
	FirstName   string
	LastName    string
	MiddleName  string
	Email       string
	Phone       string
	Language    string
	LastAction  string
	CreatedAt   string
	UpdatedAt   string
	AvatarID    string
	Roles       []*Role
	GroupIDs    []string
	Permissions []*Permission
	Avatar      *Upload
	CanUpdate   bool
	CanDelete   bool
}

func (u *User) Title() string {
	var fullName string

	firstName := strings.TrimSpace(u.FirstName)
	lastName := strings.TrimSpace(u.LastName)

	if firstName != "" && lastName != "" {
		fullName = firstName + " " + lastName
	} else if firstName != "" {
		fullName = firstName
	} else if lastName != "" {
		fullName = lastName
	}

	if fullName != "" {
		return fullName
	} else if u.Phone != "" {
		return u.Phone
	}

	return u.Email
}

func (u *User) RolesVerbose() string {
	if len(u.Roles) == 0 {
		return ""
	}
	roles := ""
	for _, role := range u.Roles {
		roles += role.Name + ", "
	}
	return roles[:len(roles)-2]
}

func (u *User) Initials() string {
	return shared.GetInitials(u.FirstName, u.LastName)
}
