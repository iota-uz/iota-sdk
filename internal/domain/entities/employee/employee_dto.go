package employee

type CreateDTO struct {
	FirstName   string
	LastName    string
	MiddleName  string
	Email       string
	Phone       string
	Salary      float64
	PositionID  uint
	Coefficient float64
}

type UpdateDTO struct {
	FirstName   string
	LastName    string
	MiddleName  string
	Email       string
	Phone       string
	Salary      float64
	PositionID  uint
	Coefficient float64
}
