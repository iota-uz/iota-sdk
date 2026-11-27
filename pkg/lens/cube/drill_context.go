package cube

import (
	"net/url"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
)

const (
	QueryFilter       = "_f"
	QueryDimension    = "_dim"
	QueryGroupBy      = "_groupby"
	QueryLeaf         = "_leaf"
	UnknownDrillValue = "__unknown__"
)

type DimensionFilter struct {
	Dimension string
	Value     string
	Values    []string
}

type DrillContext struct {
	Filters         []DimensionFilter
	ActiveDimension string
	GroupBy         string
	// Leaf explicitly requests the cube's leaf view (e.g. via _leaf=1) while
	// keeping all active drill filters and period parameters intact.
	Leaf bool
	// Request preserves non-drill query state (period comparison, external
	// controls) for cube compilation. Encode still emits only cube state.
	Request url.Values
}

type Breadcrumb struct {
	URL       string
	Dimension string
	Label     string
	Value     string
}

func ParseDrillContext(values url.Values) DrillContext {
	ctx := DrillContext{Request: cloneValues(values)}
	for _, raw := range values[QueryFilter] {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		dimension, value, ok := strings.Cut(raw, ":")
		dimension = strings.TrimSpace(dimension)
		value = strings.TrimSpace(value)
		if !ok || dimension == "" || value == "" {
			continue
		}
		ctx = ctx.withFilterValue(dimension, value)
	}
	ctx.ActiveDimension = strings.TrimSpace(values.Get(QueryDimension))
	ctx.GroupBy = strings.TrimSpace(values.Get(QueryGroupBy))
	if ctx.GroupBy == "" {
		ctx.GroupBy = ctx.ActiveDimension
	}
	leafRaw := strings.ToLower(strings.TrimSpace(values.Get(QueryLeaf)))
	ctx.Leaf = leafRaw == "1" || leafRaw == "true"
	return ctx
}

// ParseDrillContextNormalized parses cube drill state like ParseDrillContext,
// normalizing empty filter values ("dimension:") to emptyValue (defaulting to
// UnknownDrillValue) so "unknown/null" buckets are preserved instead of dropped.
func ParseDrillContextNormalized(values url.Values, emptyValue string) DrillContext {
	emptyValue = strings.TrimSpace(emptyValue)
	if emptyValue == "" {
		emptyValue = UnknownDrillValue
	}
	raw, ok := values[QueryFilter]
	if !ok {
		return ParseDrillContext(values)
	}
	normalized := make([]string, len(raw))
	changed := false
	for i, item := range raw {
		dimension, value, hasColon := strings.Cut(strings.TrimSpace(item), ":")
		if hasColon && strings.TrimSpace(dimension) != "" && strings.TrimSpace(value) == "" {
			normalized[i] = strings.TrimSpace(dimension) + ":" + emptyValue
			changed = true
			continue
		}
		normalized[i] = item
	}
	if !changed {
		return ParseDrillContext(values)
	}
	out := cloneValues(values)
	out[QueryFilter] = normalized
	return ParseDrillContext(out)
}

func (c DrillContext) Encode() url.Values {
	values := url.Values{}
	for _, filter := range c.Filters {
		dimension := strings.TrimSpace(filter.Dimension)
		filterValues := filter.values()
		if dimension == "" || len(filterValues) == 0 {
			continue
		}
		for _, value := range filterValues {
			values.Add(QueryFilter, dimension+":"+value)
		}
	}
	if trimmed := c.normalizedGroupBy(); trimmed != "" {
		values.Set(QueryGroupBy, trimmed)
	}
	if c.Leaf {
		values.Set(QueryLeaf, "1")
	}
	return values
}

func (c DrillContext) HasFilters() bool {
	return len(c.Filters) > 0
}

func (c DrillContext) WithFilter(dimension, value string) DrillContext {
	next := c.WithoutDimension(dimension)
	return next.withFilterValue(dimension, value)
}

func (c DrillContext) ToggleFilter(dimension, value string) DrillContext {
	dimension = strings.TrimSpace(dimension)
	value = strings.TrimSpace(value)
	if dimension == "" || value == "" {
		return c
	}
	next := c.clone()
	for idx, filter := range next.Filters {
		if filter.Dimension != dimension {
			continue
		}
		values := filter.values()
		for valueIdx, current := range values {
			if current != value {
				continue
			}
			values = append(values[:valueIdx], values[valueIdx+1:]...)
			if len(values) == 0 {
				next.Filters = append(next.Filters[:idx], next.Filters[idx+1:]...)
				return next
			}
			next.Filters[idx].Value = values[0]
			next.Filters[idx].Values = values
			return next
		}
		values = append(values, value)
		next.Filters[idx].Value = values[0]
		next.Filters[idx].Values = values
		return next
	}
	next.Filters = append(next.Filters, DimensionFilter{
		Dimension: dimension,
		Value:     value,
		Values:    []string{value},
	})
	return next
}

func (c DrillContext) WithoutDimension(dimension string) DrillContext {
	dimension = strings.TrimSpace(dimension)
	if dimension == "" {
		return c
	}
	next := c.clone()
	out := next.Filters[:0]
	for _, filter := range next.Filters {
		if filter.Dimension == dimension {
			continue
		}
		out = append(out, filter)
	}
	next.Filters = out
	return next
}

func (c DrillContext) PopTo(dimension string) DrillContext {
	return c.WithoutDimension(dimension)
}

func (c DrillContext) ContainsDimension(name string) bool {
	for _, filter := range c.Filters {
		if filter.Dimension == name {
			return true
		}
	}
	return false
}

func (c DrillContext) RemainingDimensions(spec CubeSpec) []DimensionSpec {
	return orderedDimensions(spec)
}

func (c DrillContext) IsLeaf(spec CubeSpec) bool {
	if len(spec.DrillPath) > 0 {
		for _, name := range spec.DrillPath {
			if !c.ContainsDimension(name) {
				return false
			}
		}
		return true
	}
	if len(spec.Dimensions) == 0 {
		return false
	}
	for _, dim := range spec.Dimensions {
		if !c.ContainsDimension(dim.Name) {
			return false
		}
	}
	return true
}

func (c DrillContext) Breadcrumbs(spec CubeSpec, baseURL string) []Breadcrumb {
	if len(c.Filters) == 0 {
		return nil
	}
	dimensions := dimensionLabels(spec)
	crumbs := make([]Breadcrumb, 0, len(c.Filters))
	for idx, filter := range c.Filters {
		part := c
		part.Leaf = false
		part.Filters = append(append([]DimensionFilter(nil), c.Filters[:idx]...), c.Filters[idx+1:]...)
		displayValue := filter.Value
		if dim, ok := spec.Dimension(filter.Dimension); ok {
			displayValue = dim.DisplayLabel(filter.Value)
		}
		crumbs = append(crumbs, Breadcrumb{
			URL:       mergeURL(baseURL, part.Encode()),
			Dimension: filter.Dimension,
			Label:     firstNonEmpty(dimensions[filter.Dimension], filter.Dimension),
			Value:     displayValue,
		})
	}
	return crumbs
}

func (c DrillContext) WithValues(values url.Values) url.Values {
	merged := cloneValues(values)
	delete(merged, QueryFilter)
	delete(merged, QueryDimension)
	delete(merged, QueryGroupBy)
	delete(merged, QueryLeaf)
	delete(merged, QueryFacet)
	delete(merged, QueryFacetSearch)
	for _, filter := range c.Filters {
		filterValues := filter.values()
		if len(filterValues) == 0 {
			continue
		}
		for _, value := range filterValues {
			merged.Add(QueryFilter, filter.Dimension+":"+value)
		}
	}
	if trimmed := c.normalizedGroupBy(); trimmed != "" {
		merged.Set(QueryGroupBy, trimmed)
	}
	if c.Leaf {
		merged.Set(QueryLeaf, "1")
	}
	return merged
}

func Strip(values url.Values) url.Values {
	clean := cloneValues(values)
	delete(clean, QueryFilter)
	delete(clean, QueryDimension)
	delete(clean, QueryGroupBy)
	delete(clean, QueryLeaf)
	delete(clean, QueryFacet)
	delete(clean, QueryFacetSearch)
	return clean
}

func drillMeta(spec CubeSpec, drillCtx DrillContext, baseURL string, remaining []DimensionSpec) *lens.DrillMeta {
	meta := &lens.DrillMeta{
		BaseURL:             baseURL,
		Dimensions:          make([]lens.DrillDimensionMeta, 0, len(spec.Dimensions)),
		Filters:             make([]lens.DrillFilterMeta, 0, len(drillCtx.Filters)),
		RemainingDimensions: make([]lens.DrillDimensionMeta, 0, len(remaining)),
		ActiveDimension:     drillCtx.normalizedGroupBy(),
		GroupBy:             drillCtx.normalizedGroupBy(),
	}
	for _, dim := range orderedDimensions(spec) {
		meta.Dimensions = append(meta.Dimensions, lens.DrillDimensionMeta{
			Name:  dim.Name,
			Label: dim.Label,
		})
	}
	for _, filter := range drillCtx.Filters {
		dim, hasDim := spec.Dimension(filter.Dimension)
		for _, value := range filter.values() {
			display := value
			if hasDim {
				display = dim.DisplayLabel(value)
			}
			meta.Filters = append(meta.Filters, lens.DrillFilterMeta{
				Dimension: filter.Dimension,
				Value:     value,
				Display:   display,
			})
		}
	}
	for _, dim := range remaining {
		meta.RemainingDimensions = append(meta.RemainingDimensions, lens.DrillDimensionMeta{
			Name:  dim.Name,
			Label: dim.Label,
		})
	}
	return meta
}

func (c DrillContext) clone() DrillContext {
	next := DrillContext{
		Filters:         make([]DimensionFilter, len(c.Filters)),
		ActiveDimension: c.ActiveDimension,
		GroupBy:         c.GroupBy,
		Leaf:            c.Leaf,
		Request:         cloneValues(c.Request),
	}
	for idx, filter := range c.Filters {
		next.Filters[idx] = filter
		next.Filters[idx].Values = append([]string(nil), filter.Values...)
	}
	return next
}

func (c DrillContext) normalizedGroupBy() string {
	if trimmed := strings.TrimSpace(c.GroupBy); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(c.ActiveDimension)
}

func (c DrillContext) withFilterValue(dimension, value string) DrillContext {
	dimension = strings.TrimSpace(dimension)
	value = strings.TrimSpace(value)
	if dimension == "" || value == "" {
		return c
	}
	next := c.clone()
	for idx, filter := range next.Filters {
		if filter.Dimension != dimension {
			continue
		}
		values := filter.values()
		for _, current := range values {
			if current == value {
				return next
			}
		}
		values = append(values, value)
		next.Filters[idx].Value = values[0]
		next.Filters[idx].Values = values
		return next
	}
	next.Filters = append(next.Filters, DimensionFilter{
		Dimension: dimension,
		Value:     value,
		Values:    []string{value},
	})
	return next
}

func (f DimensionFilter) values() []string {
	raw := f.Values
	if len(raw) == 0 && strings.TrimSpace(f.Value) != "" {
		raw = []string{f.Value}
	}
	values := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, value := range raw {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func mergeURL(baseURL string, query url.Values) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = "/"
	}
	if len(query) == 0 {
		return baseURL
	}
	separator := "?"
	if strings.ContainsRune(baseURL, '?') {
		separator = "&"
	}
	return baseURL + separator + query.Encode()
}

func cloneValues(values url.Values) url.Values {
	if values == nil {
		return url.Values{}
	}
	cloned := make(url.Values, len(values))
	for key, items := range values {
		cloned[key] = append([]string(nil), items...)
	}
	return cloned
}

func dimensionLabels(spec CubeSpec) map[string]string {
	labels := make(map[string]string, len(spec.Dimensions))
	for _, dim := range spec.Dimensions {
		labels[dim.Name] = dim.Label
	}
	return labels
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
