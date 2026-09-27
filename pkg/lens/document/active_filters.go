package document

import (
	"net/url"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
)

func buildActiveFilters(meta *lens.DrillMeta) ([]ActiveFilter, string) {
	if meta == nil || len(meta.Filters) == 0 {
		return nil, ""
	}
	labels := make(map[string]string, len(meta.Dimensions))
	for _, dimension := range meta.Dimensions {
		labels[dimension.Name] = dimension.Label
	}
	active := make([]ActiveFilter, 0, len(meta.Filters))
	for index, filter := range meta.Filters {
		remaining := append([]lens.DrillFilterMeta(nil), meta.Filters[:index]...)
		remaining = append(remaining, meta.Filters[index+1:]...)
		active = append(active, ActiveFilter{
			Dimension: filter.Dimension,
			Label:     firstNonBlank(labels[filter.Dimension], filter.Dimension) + ": " + firstNonBlank(filter.Display, filter.Value),
			Value:     filter.Value,
			RemoveURL: filterStackURL(meta.BaseURL, remaining, meta.GroupBy),
		})
	}
	return active, filterStackURL(meta.BaseURL, nil, "")
}

func filterStackURL(base string, filters []lens.DrillFilterMeta, groupBy string) string {
	target, err := url.Parse(strings.TrimSpace(base))
	if err != nil || target.IsAbs() || target.Host != "" {
		target = &url.URL{Path: "/"}
	}
	query := target.Query()
	query.Del("_f")
	query.Del("_dim")
	query.Del("_groupby")
	for _, filter := range filters {
		query.Add("_f", filter.Dimension+":"+filter.Value)
	}
	if groupBy = strings.TrimSpace(groupBy); groupBy != "" {
		query.Set("_groupby", groupBy)
	}
	target.RawQuery = query.Encode()
	return target.String()
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
