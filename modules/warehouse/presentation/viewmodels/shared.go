package viewmodels

import "strings"

type Upload struct {
	ID       string
	URL      string
	Path     string
	Hash     string
	Slug     string
	Size     string
	Mimetype string
}

type User struct {
	ID        string
	FirstName string
	LastName  string
}

func (u *User) Title() string {
	if u == nil {
		return "Unknown User"
	}
	firstName := strings.TrimSpace(u.FirstName)
	lastName := strings.TrimSpace(u.LastName)

	if firstName == "" && lastName == "" {
		if u.ID == "" {
			return "Unknown User"
		}
		return "User #" + u.ID
	}

	if firstName == "" {
		return lastName
	}

	if lastName == "" {
		return firstName
	}

	return firstName + " " + lastName
}
