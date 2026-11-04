// Package viewmodels provides this package.
package viewmodels

import (
	"html/template"
	"time"
)

type CategoryNode struct {
	Title    string
	Path     string
	Order    int
	Children []CategoryNode
}

type DocView struct {
	Title          string
	Path           string
	HTML           template.HTML
	Categories     []string
	Part           string
	Audience       string
	Updated        time.Time
	ReadingMinutes int
	Previous       *DocLink
	Next           *DocLink
	FeedbackURL    string
}

type DocLink struct {
	Title string
	Path  string
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
	// ReadingMinutes is filled from the linked article when the page renders.
	ReadingMinutes int
}
