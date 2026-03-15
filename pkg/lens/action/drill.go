package action

type DestinationKind string

const (
	DestinationDashboard DestinationKind = "dashboard"
	DestinationRaw       DestinationKind = "raw"
)

type DrillSpec struct {
	Destination DestinationKind
	PageTitle   string
	ScopeLabel  string
	LabelSource ValueSource
}
