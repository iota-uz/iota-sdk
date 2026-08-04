// Package viewmodels provides this package.
package viewmodels

import "html/template"

type CategoryNode struct {
	Title    string
	Path     string
	Children []CategoryNode
}

type DocView struct {
	Title string
	Path  string
	HTML  template.HTML
}

type SearchHit struct {
	Title   string
	Path    string
	Excerpt template.HTML
	Score   float64
}

type TaskGroup struct {
	Title string
	Tasks []HelpTask
}

type HelpTask struct {
	Title       string
	Description string
	Outcome     string
	ScreenPath  string
	ArticlePath string
}
