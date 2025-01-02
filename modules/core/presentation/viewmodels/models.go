package viewmodels

type ProjectStage struct {
	ID        string
	Name      string
	ProjectID string
	Margin    string
	Risks     string
	CreatedAt string
	UpdatedAt string
}

type Project struct {
	ID          string
	Name        string
	Description string
	CreatedAt   string
	UpdatedAt   string
}

type Currency struct {
	Code   string
	Name   string
	Symbol string
}

type Employee struct {
	ID              string
	FirstName       string
	LastName        string
	MiddleName      string
	Email           string
	Phone           string
	Salary          string
	BirthDate       string
	Tin             string
	Pin             string
	HireDate        string
	ResignationDate string
	Notes           string
	CreatedAt       string
	UpdatedAt       string
}
type Upload struct {
	ID        string
	Hash      string
	URL       string
	Mimetype  string
	Size      string
	CreatedAt string
	UpdatedAt string
}

type Tab struct {
	ID   string
	Href string
}
