package pagination

import (
	"fmt"
	"math"
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
	Total   int
	Current int
	pages   []Page
}

func (s *State) Pages() []Page {
	filler := Page{Filler: true}
	if len(s.pages) < 10 {
		return s.pages
	}
	if s.Current < 5 {
		return append(s.pages[:10], filler, s.pages[len(s.pages)-1])
	}
	if s.Current > len(s.pages)-5 {
		data := []Page{s.pages[0], filler}
		return append(data, s.pages[len(s.pages)-10:]...)
	}
	data := []Page{s.pages[0], filler}
	data = append(data, s.pages[s.Current-5:s.Current+5]...)
	return append(data, filler, s.pages[len(s.pages)-1])
}

func (s *State) PrevLink() string {
	if len(s.pages) == 0 {
		return ""
	}
	if s.Current-1 < 0 {
		return ""
	}
	return s.pages[s.Current-1].Link
}

func (s *State) NextLink() string {
	if len(s.pages) == 0 {
		return ""
	}
	if s.Current+1 >= len(s.pages) {
		return ""
	}
	return s.pages[s.Current+1].Link
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
	p := &State{
		Total:   total,
		Current: max(page-1, 0),
	}
	if total == 0 {
		return p
	}
	pages := int(math.Ceil(float64(total) / float64(limit)))
	for i := 0; i < pages; i++ {
		link := baseLink
		if i > 0 {
			link += fmt.Sprintf("?page=%d", i+1)
		}
		p.pages = append(p.pages, Page{
			Num:    i + 1,
			Link:   link,
			Active: i == max(page-1, 0),
		})
	}
	return p
}
