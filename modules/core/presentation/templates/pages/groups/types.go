package groups

import "github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"

// IndexPageProps holds the data for rendering the groups index page
type IndexPageProps struct {
	Groups  []*viewmodels.Group
	Page    int
	PerPage int
	Search  string
	HasMore bool
}
