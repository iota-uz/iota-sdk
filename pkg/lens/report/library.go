package report

import (
	"context"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Ownership states whose report a catalog entry is, relative to the reader.
type Ownership string

const (
	// OwnershipPersonal is a report the reader owns.
	OwnershipPersonal Ownership = "personal"
	// OwnershipShared is a report another person owns and shares with the reader.
	OwnershipShared Ownership = "shared"
	// OwnershipSystem is a code-authored report the application ships.
	OwnershipSystem Ownership = "system"
)

// CatalogScope narrows a catalog search by ownership.
type CatalogScope string

const (
	CatalogScopeAll      CatalogScope = "all"
	CatalogScopePersonal CatalogScope = "personal"
	// CatalogScopeShared lists every report the reader does not own: shared and system reports.
	CatalogScopeShared CatalogScope = "shared"
)

const (
	DefaultCatalogLimit = 20
	MaxCatalogLimit     = 100
)

// CatalogQuery is one page request of a report picker. Target names the
// composition being edited so the host can count existing placements.
type CatalogQuery struct {
	Search string       `json:"search,omitempty"`
	Scope  CatalogScope `json:"scope,omitempty"`
	Cursor string       `json:"cursor,omitempty"`
	Target string       `json:"target,omitempty"`
	Limit  int          `json:"limit,omitempty"`
}

type CatalogOwner struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CatalogEntry describes one report the reader may place. A host lists only
// readable reports: an inaccessible one contributes no title, count or preview.
type CatalogEntry struct {
	Owner *CatalogOwner `json:"owner,omitempty"`
	// ID is the report definition identity a placement references.
	ID       string `json:"id"`
	Revision string `json:"revision"`
	Title    string `json:"title"`
	// Description is a short, plain-text summary.
	Description string `json:"description,omitempty"`
	// Source is a display label of the data source, never its query.
	Source    string    `json:"source,omitempty"`
	Ownership Ownership `json:"ownership"`
	// Restriction is a host note the reader must see before adding the report,
	// for example a period limit or an export the source cannot offer.
	Restriction string `json:"restriction,omitempty"`
	// Members counts the panels of a compound report. The report is still one
	// choice: its datasets and member panels are never offered separately.
	Members int `json:"members,omitempty"`
	// Placed counts this report's placements in the query's target composition.
	Placed int `json:"placed,omitempty"`
}

// CatalogPage is one page of readable entries. Next is an opaque cursor for
// the following page; empty when there is none. A page carries no total, so a
// reader cannot infer the number of reports hidden from them.
type CatalogPage struct {
	Entries []CatalogEntry `json:"entries"`
	Next    string         `json:"next,omitempty"`
}

// Destination is a composition a report can be added to from an opened analysis.
type Destination struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Placed int    `json:"placed,omitempty"`
}

// Library searches the reports a reader may place. Hosts back it with any
// storage and apply authorization before returning an entry.
type Library interface {
	Search(context.Context, CatalogQuery) (CatalogPage, error)
}

// Destinations lists the compositions a reader may add a report to.
type Destinations interface {
	Destinations(ctx context.Context, reportID, search string) ([]Destination, error)
}

// Normalize applies the default scope and page size and rejects values a host
// cannot serve.
func (q CatalogQuery) Normalize() (CatalogQuery, error) {
	normalized := q
	normalized.Search = strings.TrimSpace(q.Search)
	switch normalized.Scope {
	case "":
		normalized.Scope = CatalogScopeAll
	case CatalogScopeAll, CatalogScopePersonal, CatalogScopeShared:
	default:
		return CatalogQuery{}, invalid("unknown catalog scope")
	}
	switch {
	case normalized.Limit == 0:
		normalized.Limit = DefaultCatalogLimit
	case normalized.Limit < 0 || normalized.Limit > MaxCatalogLimit:
		return CatalogQuery{}, invalid("catalog page size is out of range")
	}
	if normalized.Target != "" && !identifier.MatchString(normalized.Target) {
		return CatalogQuery{}, invalid("invalid catalog target")
	}
	return normalized, nil
}

// Includes reports whether an entry of the given ownership belongs to the scope.
func (s CatalogScope) Includes(ownership Ownership) bool {
	switch s {
	case "", CatalogScopeAll:
		return true
	case CatalogScopePersonal:
		return ownership == OwnershipPersonal
	case CatalogScopeShared:
		return ownership == OwnershipShared || ownership == OwnershipSystem
	default:
		return false
	}
}

func (e CatalogEntry) Validate() error {
	if !identifier.MatchString(e.ID) || e.Revision == "" || strings.TrimSpace(e.Title) == "" {
		return invalid("catalog entry identity, revision and title are required")
	}
	switch e.Ownership {
	case OwnershipPersonal, OwnershipShared, OwnershipSystem:
	default:
		return invalid("catalog entry ownership is required")
	}
	if e.Members < 0 || e.Placed < 0 {
		return invalid("catalog entry counts cannot be negative")
	}
	if e.Owner != nil && (e.Owner.ID == "" || strings.TrimSpace(e.Owner.Name) == "") {
		return invalid("catalog entry owner needs an identity and a name")
	}
	return nil
}

func (p CatalogPage) Validate() error {
	seen := make(map[string]bool, len(p.Entries))
	for _, entry := range p.Entries {
		if err := entry.Validate(); err != nil {
			return err
		}
		if seen[entry.ID] {
			return invalid("catalog page repeats a report")
		}
		seen[entry.ID] = true
	}
	return nil
}

// PlacementCounts counts the placements of each report in a composition.
func PlacementCounts(composition Composition) map[string]int {
	counts := map[string]int{}
	for _, row := range composition.Rows {
		for _, placement := range row {
			counts[placement.ReportID]++
		}
	}
	return counts
}

// MarkPlaced sets each entry's Placed count from the target composition.
func MarkPlaced(page CatalogPage, composition Composition) CatalogPage {
	counts := PlacementCounts(composition)
	marked := clone(page)
	for i := range marked.Entries {
		marked.Entries[i].Placed = counts[marked.Entries[i].ID]
	}
	return marked
}

// Search lists the code-authored definitions as system reports, matching the
// search text case-insensitively against the title. The cursor is an offset.
func (c Catalog) Search(_ context.Context, query CatalogQuery) (CatalogPage, error) {
	const op serrors.Op = "lens.report.Catalog.Search"
	normalized, err := query.Normalize()
	if err != nil {
		return CatalogPage{}, serrors.Wrap(op, err)
	}
	offset := 0
	if normalized.Cursor != "" {
		offset, err = strconv.Atoi(normalized.Cursor)
		if err != nil || offset < 0 {
			return CatalogPage{}, serrors.New(serrors.Invalid, "invalid catalog cursor").WithOp(op)
		}
	}
	needle := strings.ToLower(normalized.Search)
	matches := make([]CatalogEntry, 0, len(c.Definitions))
	for _, definition := range c.Definitions {
		if !normalized.Scope.Includes(OwnershipSystem) {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(definition.Title), needle) {
			continue
		}
		matches = append(matches, CatalogEntry{ID: definition.ID, Revision: definition.Revision, Title: definition.Title, Ownership: OwnershipSystem})
	}
	page := CatalogPage{Entries: []CatalogEntry{}}
	if offset >= len(matches) {
		return page, nil
	}
	end := min(offset+normalized.Limit, len(matches))
	page.Entries = matches[offset:end]
	if end < len(matches) {
		page.Next = strconv.Itoa(end)
	}
	return page, nil
}
