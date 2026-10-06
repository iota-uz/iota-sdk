// Package reporting compiles declarative reports against owner-defined semantic
// sources. Neither a plan nor catalog discovery accepts SQL from the model.
package reporting

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	ErrDefinition     = errors.New("invalid semantic definition")
	ErrPlan           = errors.New("invalid report plan")
	ErrUnknownDataset = errors.New("report dataset is not registered")
	ErrVersion        = errors.New("report definition version changed; discover the current definition")
)

type Field struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"` // string, number, date
}

// Measure supports additive sums and population counts. Ratios, distinct
// counts and balances require a separately defined aggregation contract.
type Measure struct {
	Field
	Aggregation       string `json:"aggregation"` // sum, count
	Unit              string `json:"unit"`
	CurrencyDimension string `json:"currency_dimension,omitempty"`
}

type Dataset struct {
	ID                  string    `json:"id"`
	Version             string    `json:"version"`
	Description         string    `json:"description"`
	Grain               string    `json:"grain"`
	Population          string    `json:"population"`
	DateBasis           string    `json:"date_basis"`
	Timezone            string    `json:"timezone"`
	PeriodParameterType string    `json:"period_parameter_type"` // date or timestamp
	Dimensions          []Field   `json:"dimensions"`
	Measures            []Measure `json:"measures"`
	Limitations         []string  `json:"limitations"`
	// SQL is supplied by the canonical source owner, with $1/$2 half-open
	// period bounds and its own tenant/population constraints. Never discovered.
	SourceSQL string `json:"-"`
}

type Catalog struct {
	datasets map[string]Dataset
	ids      []string
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func NewCatalog(definitions ...Dataset) (*Catalog, error) {
	c := &Catalog{datasets: map[string]Dataset{}}
	for _, d := range definitions {
		if !identifier.MatchString(d.ID) || d.Version == "" || d.SourceSQL == "" || d.Grain == "" || d.Population == "" || d.DateBasis == "" || d.Timezone == "" || (d.PeriodParameterType != "date" && d.PeriodParameterType != "timestamp") {
			return nil, fmt.Errorf("%w: dataset %s", ErrDefinition, d.ID)
		}
		if _, exists := c.datasets[d.ID]; exists {
			return nil, ErrDefinition
		}
		if _, err := time.LoadLocation(d.Timezone); err != nil {
			return nil, fmt.Errorf("%w: timezone for %s", ErrDefinition, d.ID)
		}
		fields := map[string]Field{}
		for _, f := range d.Dimensions {
			if !validField(f) {
				return nil, ErrDefinition
			}
			if _, exists := fields[f.Name]; exists {
				return nil, ErrDefinition
			}
			fields[f.Name] = f
		}
		date, exists := fields[d.DateBasis]
		if !exists || date.Type != "date" {
			return nil, ErrDefinition
		}
		for _, m := range d.Measures {
			if !validField(m.Field) || m.Type != "number" || m.Unit == "" || (m.Aggregation != "sum" && m.Aggregation != "count") {
				return nil, ErrDefinition
			}
			if _, exists := fields[m.Name]; exists {
				return nil, ErrDefinition
			}
			if m.CurrencyDimension != "" {
				f, exists := fields[m.CurrencyDimension]
				if !exists || f.Type != "string" || m.Aggregation != "sum" {
					return nil, ErrDefinition
				}
			}
			fields[m.Name] = m.Field
		}
		if len(d.Measures) == 0 {
			return nil, ErrDefinition
		}
		c.datasets[d.ID] = cloneDataset(d)
		c.ids = append(c.ids, d.ID)
	}
	sort.Strings(c.ids)
	return c, nil
}

func validField(f Field) bool {
	return identifier.MatchString(f.Name) && (f.Type == "string" || f.Type == "number" || f.Type == "date")
}
func cloneDataset(d Dataset) Dataset {
	d.Dimensions = append([]Field(nil), d.Dimensions...)
	d.Measures = append([]Measure(nil), d.Measures...)
	d.Limitations = append([]string(nil), d.Limitations...)
	return d
}

func (c *Catalog) Dataset(id string) (Dataset, error) {
	d, ok := c.datasets[id]
	if !ok {
		return Dataset{}, fmt.Errorf("%w: %q; discover a registered definition; do not substitute another methodology", ErrUnknownDataset, id)
	}
	return cloneDataset(d), nil
}

// Search is bounded regardless of the number of registered definitions.
// Exact IDs are retrieved separately; SQL is excluded from JSON serialization.
func (c *Catalog) Search(query string, offset, limit int) ([]Dataset, int, error) {
	if offset < 0 || limit < 1 || limit > 20 || len(query) > 256 {
		return nil, 0, ErrPlan
	}
	words := strings.Fields(strings.ToLower(query))
	matched := []Dataset{}
	total := 0
	for _, id := range c.ids {
		d := c.datasets[id]
		data := strings.ToLower(d.ID + " " + d.Description + " " + d.Population)
		for _, m := range d.Measures {
			data += " " + strings.ToLower(m.Name+" "+m.Description)
		}
		matches := true
		for _, word := range words {
			if !strings.Contains(data, word) {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		if total >= offset && len(matched) < limit {
			matched = append(matched, cloneDataset(d))
		}
		total++
	}
	return matched, total, nil
}
