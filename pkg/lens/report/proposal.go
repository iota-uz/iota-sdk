package report

import "strings"

// Proposal is an externally prepared replacement of a draft composition, for
// example from an assistant. Base is the opaque token of the draft state it was
// prepared against; the editor treats a proposal as stale once the draft moved
// on and never applies it. A proposal changes placements only: it cannot alter
// a shared report definition.
type Proposal struct {
	ID          string      `json:"id"`
	Base        string      `json:"base"`
	Summary     string      `json:"summary,omitempty"`
	Composition Composition `json:"composition"`
}

func (p Proposal) Validate() error {
	if !identifier.MatchString(p.ID) || strings.TrimSpace(p.Base) == "" {
		return invalid("proposal identity and base are required")
	}
	return p.Composition.Validate()
}
