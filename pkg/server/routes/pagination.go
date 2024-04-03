package routes

import (
	"fmt"
	"math"
)

type Page struct {
	Num    int
	Link   string
	Active bool
}

const baseAnchorClasses = "flex items-center justify-center px-4 h-10 border border-gray-300 dark:border-gray-700"
const baseAnchorBg = "bg-white leading-tight hover:bg-gray-100 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-white text-gray-500 hover:text-gray-700"
const activeAnchorClasses = "text-blue-600 bg-blue-50 hover:bg-blue-100 hover:text-blue-700 dark:bg-gray-700 dark:text-white"
const disabledAnchorClasses = "text-gray-400 dark:text-gray-700 pointer-events-none"

func (p *Page) Classes() string {
	if p.Active {
		return baseAnchorClasses + " " + activeAnchorClasses
	}
	return baseAnchorClasses + " " + baseAnchorBg
}

type Pagination struct {
	Total   int
	Current int
	Pages   []Page
}

func (p *Pagination) PrevLink() string {
	if len(p.Pages) == 0 {
		return ""
	}
	if p.Current-1 < 0 {
		return ""
	}
	return p.Pages[p.Current-1].Link
}

func (p *Pagination) NextLink() string {
	if len(p.Pages) == 0 {
		return ""
	}
	if p.Current+1 >= len(p.Pages) {
		return ""
	}
	return p.Pages[p.Current+1].Link
}

func (p *Pagination) PrevLinkClasses() string {
	base := baseAnchorClasses + " rounded-l-lg "
	if p.PrevLink() == "" {
		return base + disabledAnchorClasses
	}
	return base + baseAnchorBg
}

func (p *Pagination) NextLinkClasses() string {
	base := baseAnchorClasses + " rounded-r-lg "
	if p.NextLink() == "" {
		return base + disabledAnchorClasses
	}
	return base + baseAnchorBg
}

func NewPagination(baseLink string, page, total, limit int) *Pagination {
	p := &Pagination{
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
		p.Pages = append(p.Pages, Page{
			Num:    i + 1,
			Link:   link,
			Active: i == page,
		})
	}
	return p
}
