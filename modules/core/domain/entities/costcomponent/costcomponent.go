package costcomponent

var (
	HoursInMonth float64 = 168
)

type BillableHourEntity struct {
	Name string
}

type ExpenseComponent struct {
	Purpose string
	Value   float64
}

type CostComponent struct {
	Purpose string
	Monthly float64
	Hourly  float64
}

type UnifiedHourlyRateResult struct {
	Entity     BillableHourEntity
	Components []CostComponent
}
