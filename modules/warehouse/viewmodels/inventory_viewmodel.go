package viewmodels

type Check struct {
	ID         string
	Type       string
	Status     string
	Name       string
	Results    []*CheckResult
	CreatedAt  string
	FinishedAt string
}

type CheckResult struct {
	ID               string
	PositionID       string
	ExpectedQuantity string
	ActualQuantity   string
	Difference       string
	CreatedAt        string
}
