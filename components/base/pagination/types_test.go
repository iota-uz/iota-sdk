package pagination

import (
	"net/url"
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
