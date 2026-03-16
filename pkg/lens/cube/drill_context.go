package cube

import (
	"net/url"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
)

const (
	QueryFilter    = "_f"
	QueryDimension = "_dim"
)

type DimensionFilter struct {
	Dimension string
	Value     string
}

type DrillContext struct {
	Filters          []DimensionFilter
	ActiveDimension  string
}

type Breadcrumb struct {
	URL       string
	Dimension string
	Label     string
	Value     string
}

func ParseDrillContext(values url.Values) DrillContext {
	ctx := DrillContext{}
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
		ctx.Filters = append(ctx.Filters, DimensionFilter{
			Dimension: dimension,
			Value:     value,
		})
	}
	ctx.ActiveDimension = strings.TrimSpace(values.Get(QueryDimension))
	return ctx
}

func (c DrillContext) Encode() url.Values {
	values := url.Values{}
	for _, filter := range c.Filters {
		if strings.TrimSpace(filter.Dimension) == "" || strings.TrimSpace(filter.Value) == "" {
			continue
		}
		values.Add(QueryFilter, filter.Dimension+":"+filter.Value)
	}
	if trimmed := strings.TrimSpace(c.ActiveDimension); trimmed != "" {
		values.Set(QueryDimension, trimmed)
	}
	return values
}

func (c DrillContext) HasFilters() bool {
	return len(c.Filters) > 0
}

func (c DrillContext) WithFilter(dimension, value string) DrillContext {
	next := DrillContext{Filters: append([]DimensionFilter(nil), c.Filters...)}
	for idx, filter := range next.Filters {
		if filter.Dimension == dimension {
			next.Filters[idx].Value = value
			next.Filters = next.Filters[:idx+1]
			return next
		}
	}
	next.Filters = append(next.Filters, DimensionFilter{
		Dimension: dimension,
		Value:     value,
	})
	return next
}

func (c DrillContext) PopTo(dimension string) DrillContext {
	if strings.TrimSpace(dimension) == "" {
		return DrillContext{}
	}
	for idx, filter := range c.Filters {
		if filter.Dimension == dimension {
			return DrillContext{Filters: append([]DimensionFilter(nil), c.Filters[:idx]...)}
		}
	}
	return c
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
	dimensions := orderedDimensions(spec)
	remaining := make([]DimensionSpec, 0, len(dimensions))
	for _, dim := range dimensions {
		if c.ContainsDimension(dim.Name) {
			continue
		}
		remaining = append(remaining, dim)
	}
	return remaining
}

func (c DrillContext) IsLeaf(spec CubeSpec) bool {
	return len(c.RemainingDimensions(spec)) == 0
}

func (c DrillContext) Breadcrumbs(spec CubeSpec, baseURL string) []Breadcrumb {
	if len(c.Filters) == 0 {
		return nil
	}
	dimensions := dimensionLabels(spec)
	crumbs := make([]Breadcrumb, 0, len(c.Filters))
	for idx, filter := range c.Filters {
		part := DrillContext{Filters: append([]DimensionFilter(nil), c.Filters[:idx]...)}
		crumbs = append(crumbs, Breadcrumb{
			URL:       mergeURL(baseURL, part.Encode()),
			Dimension: filter.Dimension,
			Label:     firstNonEmpty(dimensions[filter.Dimension], filter.Dimension),
			Value:     filter.Value,
		})
	}
	return crumbs
}

func (c DrillContext) WithValues(values url.Values) url.Values {
	merged := cloneValues(values)
	delete(merged, QueryFilter)
	delete(merged, QueryDimension)
	for _, filter := range c.Filters {
		merged.Add(QueryFilter, filter.Dimension+":"+filter.Value)
	}
	if trimmed := strings.TrimSpace(c.ActiveDimension); trimmed != "" {
		merged.Set(QueryDimension, trimmed)
	}
	return merged
}

func Strip(values url.Values) url.Values {
	clean := cloneValues(values)
	delete(clean, QueryFilter)
	delete(clean, QueryDimension)
	return clean
}

func drillMeta(spec CubeSpec, drillCtx DrillContext, baseURL string, remaining []DimensionSpec) *lens.DrillMeta {
	meta := &lens.DrillMeta{
		BaseURL:             baseURL,
		Dimensions:          make([]lens.DrillDimensionMeta, 0, len(spec.Dimensions)),
		RemainingDimensions: make([]lens.DrillDimensionMeta, 0, len(remaining)),
		ActiveDimension:     drillCtx.ActiveDimension,
	}
	for _, dim := range orderedDimensions(spec) {
		meta.Dimensions = append(meta.Dimensions, lens.DrillDimensionMeta{
			Name:  dim.Name,
			Label: dim.Label,
		})
	}
	for _, dim := range remaining {
		meta.RemainingDimensions = append(meta.RemainingDimensions, lens.DrillDimensionMeta{
			Name:  dim.Name,
			Label: dim.Label,
		})
	}
	return meta
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
