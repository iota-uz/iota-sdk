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

const baseAnchorClasses = "flex items-center justify-center px-4 h-10 border border-gray-300 dark:border-gray-700"
const baseAnchorBg = "bg-white leading-tight hover:bg-gray-100 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-white text-gray-500 hover:text-gray-700"
const activeAnchorClasses = "text-blue-600 bg-blue-50 hover:bg-blue-100 hover:text-blue-700 dark:bg-gray-700 dark:text-white"
const disabledAnchorClasses = "text-gray-400 dark:text-gray-700 pointer-events-none"

func (p *Page) Classes() string {
	if p.Filler {
		return baseAnchorClasses + " " + disabledAnchorClasses
	}
	if p.Active {
		return baseAnchorClasses + " " + activeAnchorClasses
	}
	return baseAnchorClasses + " " + baseAnchorBg
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
	base := baseAnchorClasses + " rounded-l-lg "
	if s.PrevLink() == "" {
		return base + disabledAnchorClasses
	}
	return base + baseAnchorBg
}

func (s *State) NextLinkClasses() string {
	base := baseAnchorClasses + " rounded-r-lg "
	if s.NextLink() == "" {
		return base + disabledAnchorClasses
	}
	return base + baseAnchorBg
}

func New(baseLink string, page, total, limit int) *State {
	p := &State{
		Total:   total,
		Current: page,
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
			Active: i == page,
		})
	}
	return p
}
