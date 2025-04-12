package viewmodels

import (
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type Passport struct {
	ID             string
	Series         string
	Number         string
	FirstName      string
	LastName       string
	MiddleName     string
	Gender         string
	BirthDate      string
	BirthPlace     string
	Nationality    string
	PassportType   string
	IssuedAt       string
	IssuedBy       string
	IssuingCountry string
	ExpiresAt      string
}

type Client struct {
	ID          string
	FirstName   string
	LastName    string
	MiddleName  string
	Phone       string
	Email       string
	Address     string
	Passport    Passport
	Pin         string
	CountryCode string
	DateOfBirth string
	Gender      string
	CreatedAt   string
	UpdatedAt   string
}

func (c *Client) FullName() string {
	var result []string
	if c.FirstName != "" {
		result = append(result, c.FirstName)
	}
	if c.LastName != "" {
		result = append(result, c.LastName)
	}
	if len(result) == 0 {
		return "+" + c.Phone
	}
	return strings.Join(result, " ")
}

func (c *Client) Initials() string {
	return shared.GetInitials(c.FirstName, c.LastName)
}
