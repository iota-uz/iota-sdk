package pagination

import (
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewWithQueryKeepsConditionsOnEveryPage(t *testing.T) {
	t.Parallel()
	q := url.Values{"f": {"project:is:p1", "factual_end:empty:"}, "fb": {"1"}, "Search": {"этап"}, "limit": {"2"}, "page": {"1"}}
	s := NewWithQuery("/project-stages", q, 1, 5, 2)
	pages := s.Pages()
	assert.Len(t, pages, 3)
	assert.Equal(t, "/project-stages?Search=%D1%8D%D1%82%D0%B0%D0%BF&f=project%3Ais%3Ap1&f=factual_end%3Aempty%3A&fb=1&limit=2", pages[0].Link)
	assert.Equal(t, "/project-stages?Search=%D1%8D%D1%82%D0%B0%D0%BF&f=project%3Ais%3Ap1&f=factual_end%3Aempty%3A&fb=1&limit=2&page=2", s.NextLink())
	assert.Equal(t, "/project-stages", NewWithQuery("/project-stages", url.Values{}, 1, 5, 2).Pages()[0].Link)
}

func TestNewWithQueryLargePageCount(t *testing.T) {
	const total = 10_000
	query := url.Values{
		"f":     {"project:is:p1", "factual_end:empty:"},
		"limit": {"1"},
		"page":  {"500", "600"},
	}
	var state *State
	allocations := testing.AllocsPerRun(1, func() {
		state = NewWithQuery("/project-stages", query, 500, total, 1)
	})
	assert.Less(t, allocations, float64(100))
	query["f"][0] = "changed"

	assertLink := func(link string, page string) {
		t.Helper()
		parsed, err := url.Parse(link)
		if !assert.NoError(t, err) {
			return
		}
		want := url.Values{"f": {"project:is:p1", "factual_end:empty:"}, "limit": {"1"}}
		if page != "" {
			want.Set("page", page)
		}
		assert.Equal(t, "/project-stages", parsed.Path)
		assert.Equal(t, want, parsed.Query())
	}

	pages := state.Pages()
	nums := make([]int, 0, len(pages))
	for _, page := range pages {
		nums = append(nums, page.Num)
		assert.Equal(t, page.Num == 0, page.Filler)
		assert.Equal(t, page.Num == 500, page.Active)
		if !page.Filler {
			param := ""
			if page.Num > 1 {
				param = strconv.Itoa(page.Num)
			}
			assertLink(page.Link, param)
		}
	}
	assert.Equal(t, []int{1, 0, 495, 496, 497, 498, 499, 500, 501, 502, 503, 504, 0, total}, nums)
	assertLink(state.PrevLink(), "499")
	assertLink(state.NextLink(), "501")
}
