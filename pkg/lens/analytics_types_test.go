package lens

import (
	"reflect"
	"testing"
)

func TestAnalyticsPieChartOptions(t *testing.T) {
	tests := []struct {
		name       string
		categories []AnalyticsCategory
		dataType   string
		want       map[string]interface{}
	}{
		{
			name:       "empty categories",
			categories: []AnalyticsCategory{},
			dataType:   "revenue",
			want:       map[string]interface{}{},
		},
		{
			name: "categories with zero values",
			categories: []AnalyticsCategory{
				{Name: "Category1", Value: 0, Color: "#FF0000"},
				{Name: "Category2", Value: 0, Color: "#00FF00"},
			},
			dataType: "revenue",
			want:     map[string]interface{}{},
		},
		{
			name: "categories with valid values",
			categories: []AnalyticsCategory{
				{Name: "Category1", Value: 100, Color: "#FF0000"},
				{Name: "Category2", Value: 200, Color: "#00FF00"},
				{Name: "Category3", Value: 0, Color: "#0000FF"}, // Should be filtered out
			},
			dataType: "revenue",
			want: map[string]interface{}{
				"series": []float64{100, 200},
				"labels": []string{"Category1", "Category2"},
				"colors": []string{"#FF0000", "#00FF00"},
			},
		},
		{
			name: "single valid category",
			categories: []AnalyticsCategory{
				{Name: "OnlyCategory", Value: 500, Color: "#123456"},
			},
			dataType: "count",
			want: map[string]interface{}{
				"series": []float64{500},
				"labels": []string{"OnlyCategory"},
				"colors": []string{"#123456"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyticsPieChartOptions(tt.categories, tt.dataType)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AnalyticsPieChartOptions() = %v, want %v", got, tt.want)
			}
		})
	}
}
