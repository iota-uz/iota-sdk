package viewmodels

type AcceptanceDocument struct {
	ID          string
	Kind        string
	Number      string
	Date        string
	Amount      string
	Status      string
	Description string
	CanSign     bool
	CanCancel   bool
}
