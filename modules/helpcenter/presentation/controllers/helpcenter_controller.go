package controllers

import (
	"errors"
	"net/http"
	"path/filepath"
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
}

func NewHelpCenterController(cfg HelpCenterControllerConfig) application.Controller {
	return &HelpCenterController{
		basePath:       cfg.BasePath,
		contentService: cfg.ContentService,
		renderer:       cfg.Renderer,
		searcher:       cfg.Searcher,
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
	router.HandleFunc("/doc/{path:.*}", c.doc).Methods(http.MethodGet)
}

func (c *HelpCenterController) index(w http.ResponseWriter, r *http.Request) {
	tree, err := c.contentService.Tree(r.Context())
	if err != nil {
		c.renderError(w, r, err)
		return
	}
	doc, err := c.contentService.DefaultDocument(r.Context())
	if err != nil && !errors.Is(err, services.ErrDocumentNotFound) {
		c.renderError(w, r, err)
		return
	}

	props := help.IndexProps{
		BasePath:        c.basePath,
		Tree:            tree,
		SearchAvailable: c.searchAvailable(),
		Locale:          c.contentService.Locale(r.Context()),
	}
	if doc != nil {
		docView, err := c.toDocView(doc)
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
	docView, err := c.toDocView(doc)
	if err != nil {
		c.renderError(w, r, err)
		return
	}
	if htmx.WantsFullPage(r) {
		tree, err := c.contentService.Tree(r.Context())
		if err != nil {
			c.renderError(w, r, err)
			return
		}
		templ.Handler(help.Index(help.IndexProps{
			BasePath:        c.basePath,
			Tree:            tree,
			Doc:             docView,
			SearchAvailable: c.searchAvailable(),
			Locale:          c.contentService.Locale(r.Context()),
		})).ServeHTTP(w, r)
		return
	}
	templ.Handler(help.Doc(help.DocProps{Doc: docView})).ServeHTTP(w, r)
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

func (c *HelpCenterController) toDocView(doc *services.Document) (*viewmodels.DocView, error) {
	html, err := c.renderer.Render(doc.Content)
	if err != nil {
		return nil, err
	}
	return &viewmodels.DocView{Title: doc.Title, Path: doc.Path, HTML: html}, nil
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
			Excerpt: result.Excerpt,
			Score:   result.Score,
		})
	}
	return hits
}

func (c *HelpCenterController) renderError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, services.ErrDocumentNotFound) || errors.Is(err, services.ErrInvalidPath) {
		status = http.StatusNotFound
	}
	logrus.WithError(serrors.E("HelpCenterController", err)).Error("failed to render help center")
	w.WriteHeader(status)
	templ.Handler(help.Error(status)).ServeHTTP(w, r)
}
