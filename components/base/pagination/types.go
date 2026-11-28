package pagination

import (
	"maps"
	"net/url"
	"slices"
	"strconv"
)

type Page struct {
	Num    int
	Link   string
	Filler bool
	Active bool
}

func (p *Page) Classes() string {
	if p.Active {
		return "bg-brand-500 text-white"
	}
	if p.Filler {
		return "opacity-70 pointer-events-none"
	}
	return ""
}

type State struct {
	Total     int
	Current   int
	pageCount int
	baseLink  string
	query     url.Values
}

func (s *State) TotalStr() string {
	return strconv.Itoa(s.Total)
}

func (s *State) Pages() []Page {
	if s.pageCount == 0 {
		return nil
	}

	pages := make([]Page, 0, min(s.pageCount, 14))
	appendRange := func(start, end int) {
		for i := start; i < end; i++ {
			pages = append(pages, Page{
				Num:    i + 1,
				Link:   s.pageLink(i),
				Active: i == s.Current,
			})
		}
	}

	filler := Page{Filler: true}
	switch {
	case s.pageCount < 10:
		appendRange(0, s.pageCount)
	case s.Current < 5:
		appendRange(0, 10)
		pages = append(pages, filler)
		appendRange(s.pageCount-1, s.pageCount)
	case s.Current > s.pageCount-5:
		appendRange(0, 1)
		pages = append(pages, filler)
		appendRange(s.pageCount-10, s.pageCount)
	default:
		appendRange(0, 1)
		pages = append(pages, filler)
		appendRange(s.Current-5, s.Current+5)
		pages = append(pages, filler)
		appendRange(s.pageCount-1, s.pageCount)
	}
	return pages
}

func (s *State) pageLink(index int) string {
	if len(s.query) == 0 {
		if index == 0 {
			return s.baseLink
		}
		return s.baseLink + "?page=" + strconv.Itoa(index+1)
	}

	query := s.query
	if index > 0 {
		query = maps.Clone(query)
		query.Set("page", strconv.Itoa(index+1))
	}
	if encoded := query.Encode(); encoded != "" {
		return s.baseLink + "?" + encoded
	}
	return s.baseLink
}

func (s *State) PrevLink() string {
	if s.Current <= 0 || s.Current > s.pageCount {
		return ""
	}
	return s.pageLink(s.Current - 1)
}

func (s *State) NextLink() string {
	if s.Current < 0 || s.Current >= s.pageCount-1 {
		return ""
	}
	return s.pageLink(s.Current + 1)
}

func (s *State) PrevLinkClasses() string {
	if s.PrevLink() == "" {
		return "opacity-70 pointer-events-none"
	}
	return ""
}

func (s *State) NextLinkClasses() string {
	if s.NextLink() == "" {
		return "opacity-70 pointer-events-none"
	}
	return ""
}

func New(baseLink string, page, total, limit int) *State {
	s := &State{
		Total:    total,
		Current:  max(page, 1) - 1,
		baseLink: baseLink,
	}
	if total > 0 && limit > 0 {
		s.pageCount = (total-1)/limit + 1
	}
	return s
}

// NewWithQuery preserves the query parameters other than "page" in pagination links.
func NewWithQuery(baseLink string, query url.Values, page, total, limit int) *State {
	s := New(baseLink, page, total, limit)
	s.query = make(url.Values, len(query))
	for key, values := range query {
		if key != "page" {
			s.query[key] = slices.Clone(values)
		}
	}
	return s
}
