package viewmodels

type User struct {
	ID         string
	FirstName  string
	LastName   string
	MiddleName string
	Email      string
	UILanguage string
	CreatedAt  string
	UpdatedAt  string
}

func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName + " " + u.MiddleName
}
