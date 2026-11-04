// Package controllers provides this package.
package controllers

import (
	"context"
	"errors"
	"html/template"
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/helpcenter/presentation/templates/pages/help"
	"github.com/iota-uz/iota-sdk/modules/helpcenter/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/helpcenter/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/markdown"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/microcosm-cc/bluemonday"
	"github.com/sirupsen/logrus"
)

type HelpCenterControllerConfig struct {
	BasePath       string
	ContentService *services.ContentService
	Renderer       markdown.Renderer
	Searcher       kb.KBSearcher
}

type HelpCenterController struct {
	basePath       string
	contentService *services.ContentService
	renderer       markdown.Renderer
	searcher       kb.KBSearcher
	snippetPolicy  *bluemonday.Policy
}

func NewHelpCenterController(cfg HelpCenterControllerConfig) application.Controller {
	return &HelpCenterController{
		basePath:       cfg.BasePath,
		contentService: cfg.ContentService,
		renderer:       cfg.Renderer,
		searcher:       cfg.Searcher,
		snippetPolicy:  bluemonday.StrictPolicy().AllowElements("mark"),
	}
}

func (c *HelpCenterController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("helpcenter", 0, application.Route("", c.basePath))
}

func (c *HelpCenterController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.WithPageContext(),
		middleware.NavItems(),
	)
	router.HandleFunc("", c.index).Methods(http.MethodGet)
	router.HandleFunc("/", c.index).Methods(http.MethodGet)
	router.HandleFunc("/search", c.search).Methods(http.MethodGet)
	router.HandleFunc("/media/{path:.*}", c.media).Methods(http.MethodGet)
	router.HandleFunc("/doc/{path:.*}", c.doc).Methods(http.MethodGet)
}

func (c *HelpCenterController) index(w http.ResponseWriter, r *http.Request) {
	tree, err := c.contentService.Tree(r.Context())
	if err != nil {
		c.renderError(w, r, err)
		return
	}
	tasks := c.tasksWithReadingTime(r.Context())

	props := help.IndexProps{
		BasePath:        c.basePath,
		Tree:            tree,
		Tasks:           tasks,
		SearchAvailable: c.searchAvailable(),
		Locale:          c.contentService.Locale(r.Context()),
		FeedbackURL:     c.contentService.FeedbackURL(""),
	}
	if len(tasks) == 0 {
		doc, err := c.contentService.DefaultDocument(r.Context())
		if err != nil && !errors.Is(err, services.ErrDocumentNotFound) {
			c.renderError(w, r, err)
			return
		}
		if doc == nil {
			templ.Handler(help.Index(props)).ServeHTTP(w, r)
			return
		}
		docView, err := c.toDocView(doc, c.navigationTree(doc, tree))
		if err != nil {
			c.renderError(w, r, err)
			return
		}
		props.Doc = docView
	}
	templ.Handler(help.Index(props)).ServeHTTP(w, r)
}

func (c *HelpCenterController) doc(w http.ResponseWriter, r *http.Request) {
	docPath := mux.Vars(r)["path"]
	doc, err := c.contentService.Get(r.Context(), docPath)
	if err != nil {
		c.renderError(w, r, err)
		return
	}
	fullPage := htmx.WantsFullPage(r)
	tree, err := c.contentService.Tree(r.Context())
	if err != nil {
		if fullPage {
			c.renderError(w, r, err)
			return
		}
		logrus.WithError(serrors.Wrap("HelpCenterController.doc", err)).Warn("help center article rendered without navigation")
	}
	docView, err := c.toDocView(doc, c.navigationTree(doc, tree))
	if err != nil {
		c.renderError(w, r, err)
		return
	}
	if fullPage {
		templ.Handler(help.Index(help.IndexProps{
			BasePath:        c.basePath,
			Tree:            tree,
			Doc:             docView,
			SearchAvailable: c.searchAvailable(),
			Locale:          c.contentService.Locale(r.Context()),
			FeedbackURL:     c.contentService.FeedbackURL(""),
		})).ServeHTTP(w, r)
		return
	}
	templ.Handler(help.Doc(help.DocProps{BasePath: c.basePath, Doc: docView})).ServeHTTP(w, r)
}

func (c *HelpCenterController) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	props := help.SearchResultsProps{BasePath: c.basePath, Query: q, Available: c.searchAvailable()}
	if props.Available && q != "" {
		results, err := c.searcher.Search(r.Context(), q, kb.SearchOptions{TopK: 8})
		if err != nil {
			c.renderError(w, r, err)
			return
		}
		props.Results = c.mapSearchResults(results, c.contentService.Locale(r.Context()))
	}
	templ.Handler(help.SearchResults(props)).ServeHTTP(w, r)
}

func (c *HelpCenterController) media(w http.ResponseWriter, r *http.Request) {
	media, err := c.contentService.Media(r.Context(), mux.Vars(r)["path"])
	if err != nil {
		if errors.Is(err, services.ErrMediaNotFound) || errors.Is(err, services.ErrInvalidPath) {
			http.NotFound(w, r)
			return
		}
		logrus.WithError(serrors.Wrap("HelpCenterController.media", err)).Error("failed to serve help center media")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(media.Path)))
	if contentType == "" {
		contentType = http.DetectContentType(media.Content)
	}
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = mediaType
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Add("Vary", "Cookie")
	w.Header().Add("Vary", "Accept-Language")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(media.Content)
}

func (c *HelpCenterController) navigationTree(doc *services.Document, tree []viewmodels.CategoryNode) []viewmodels.CategoryNode {
	if _, found := services.Locate(tree, doc.Path); !found {
		if fallback, err := c.contentService.DefaultTree(); err == nil {
			if _, found := services.Locate(fallback, doc.Path); found {
				return fallback
			}
		}
	}
	return tree
}

func (c *HelpCenterController) toDocView(doc *services.Document, tree []viewmodels.CategoryNode) (*viewmodels.DocView, error) {
	html, err := c.renderer.Render(doc.Content)
	if err != nil {
		return nil, err
	}
	nav, _ := services.Locate(tree, doc.Path)
	return &viewmodels.DocView{
		Title:          doc.Title,
		Path:           doc.Path,
		HTML:           wrapTables(html),
		Categories:     nav.Categories,
		Part:           doc.Meta.Part,
		Audience:       doc.Meta.Audience,
		Updated:        doc.Meta.Updated,
		ReadingMinutes: doc.Meta.ReadingMinutes,
		Previous:       docLink(nav.Previous),
		Next:           docLink(nav.Next),
		FeedbackURL:    c.contentService.FeedbackURL(doc.Path),
	}, nil
}

func docLink(node *viewmodels.CategoryNode) *viewmodels.DocLink {
	if node == nil {
		return nil
	}
	return &viewmodels.DocLink{Title: node.Title, Path: node.Path}
}

var tableOpenTag = regexp.MustCompile(`<table(?:\s[^>]*)?>`)

// wrapTables gives every rendered table its own horizontal scroll container,
// so only a table wider than the article scrolls.
func wrapTables(html template.HTML) template.HTML {
	out := tableOpenTag.ReplaceAllString(string(html), `<div class="help-table-wrap">$0`)
	return template.HTML(strings.ReplaceAll(out, "</table>", "</table></div>"))
}

func (c *HelpCenterController) tasksWithReadingTime(ctx context.Context) []viewmodels.TaskGroup {
	groups := c.contentService.Tasks(ctx)
	result := make([]viewmodels.TaskGroup, len(groups))
	for i, group := range groups {
		result[i] = viewmodels.TaskGroup{Title: group.Title, Tasks: slices.Clone(group.Tasks)}
		for j, task := range result[i].Tasks {
			if task.ArticlePath == "" {
				continue
			}
			if doc, err := c.contentService.Get(ctx, task.ArticlePath); err == nil {
				result[i].Tasks[j].ReadingMinutes = doc.Meta.ReadingMinutes
			}
		}
	}
	return result
}

func (c *HelpCenterController) searchAvailable() bool {
	return c.searcher != nil && c.searcher.IsAvailable()
}

func (c *HelpCenterController) mapSearchResults(results []kb.SearchResult, locale string) []viewmodels.SearchHit {
	hits := make([]viewmodels.SearchHit, 0, len(results))
	for _, result := range results {
		path := filepath.ToSlash(result.Document.Path)
		if !strings.Contains(path, "/"+locale+"/") && !strings.HasPrefix(path, locale+"/") {
			continue
		}
		path = strings.TrimPrefix(path, locale+"/")
		if idx := strings.Index(path, "/"+locale+"/"); idx >= 0 {
			path = path[idx+len(locale)+2:]
		}
		if path == "" {
			continue
		}
		hits = append(hits, viewmodels.SearchHit{
			Title:   result.Document.Title,
			Path:    path,
			Excerpt: c.sanitizeSearchExcerpt(result),
			Score:   result.Score,
		})
	}
	return hits
}

func (c *HelpCenterController) sanitizeSearchExcerpt(result kb.SearchResult) template.HTML {
	excerpt := strings.TrimSpace(result.Excerpt)
	if excerpt == "" && len(result.Fragments) > 0 {
		excerpt = strings.TrimSpace(result.Fragments[0])
	}
	if excerpt == "" {
		excerpt = strings.TrimSpace(result.Document.Content)
		if len(excerpt) > 180 {
			excerpt = excerpt[:180] + "..."
		}
	}
	return template.HTML(c.snippetPolicy.Sanitize(excerpt))
}

func (c *HelpCenterController) renderError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, services.ErrDocumentNotFound) || errors.Is(err, services.ErrInvalidPath) {
		status = http.StatusNotFound
	}
	logrus.WithError(serrors.Wrap("HelpCenterController", err)).Error("failed to render help center")
	w.WriteHeader(status)
	templ.Handler(help.Error(status)).ServeHTTP(w, r)
}
