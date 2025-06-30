package controllers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	_ "github.com/go-faster/errors"
	"github.com/gorilla/mux"

	_ "github.com/iota-uz/iota-sdk/components/base/pagination"
	messagetemplate "github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message-template"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/mappers"
	_ "github.com/iota-uz/iota-sdk/modules/crm/presentation/mappers"
	msgtui "github.com/iota-uz/iota-sdk/modules/crm/presentation/templates/pages/message-templates"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	_ "github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type MessageTemplateController struct {
	basePath        string
	app             application.Application
	templateService *services.MessageTemplateService
}

func NewMessageTemplateController(app application.Application, basePath string) application.Controller {
	return &MessageTemplateController{
		app:             app,
		basePath:        basePath,
		templateService: app.Service(services.MessageTemplateService{}).(*services.MessageTemplateService),
	}
}

func (c *MessageTemplateController) Key() string {
	return c.basePath
}

func (c *MessageTemplateController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}
	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", c.List).Methods(http.MethodGet)
	getRouter.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
	getRouter.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())
	setRouter.HandleFunc("", c.Create).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Update).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
}

func (c *MessageTemplateController) List(w http.ResponseWriter, r *http.Request) {
	templateEntities, err := c.templateService.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &msgtui.IndexPageProps{
		BaseURL:   c.basePath,
		NewURL:    fmt.Sprintf("%s/new", c.basePath),
		Templates: mapping.MapViewModels(templateEntities, mappers.MessageTemplateToViewModel),
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	if isHxRequest {
		templ.Handler(msgtui.TemplatesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(msgtui.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *MessageTemplateController) GetNew(w http.ResponseWriter, r *http.Request) {
	props := &msgtui.CreatePageProps{
		SaveURL:  c.basePath,
		Template: &viewmodels.MessageTemplate{},
		Errors:   map[string]string{},
	}
	templ.Handler(msgtui.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *MessageTemplateController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templateEntity, err := c.templateService.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, persistence.ErrMessageTemplateNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	props := &msgtui.EditPageProps{
		SaveURL:   fmt.Sprintf("%s/%d", c.basePath, id),
		DeleteURL: fmt.Sprintf("%s/%d", c.basePath, id),
		Errors:    map[string]string{},
		Template:  mappers.MessageTemplateToViewModel(templateEntity),
	}
	templ.Handler(msgtui.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *MessageTemplateController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&messagetemplate.CreateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		props := &msgtui.CreatePageProps{
			Errors: errorsMap,
			Template: &viewmodels.MessageTemplate{
				Template: dto.Template,
			},
			SaveURL: c.basePath,
		}
		templ.Handler(msgtui.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if _, err := c.templateService.Create(r.Context(), dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *MessageTemplateController) Update(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dto, err := composables.UseForm(&messagetemplate.UpdateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		props := &msgtui.EditPageProps{
			Errors: errorsMap,
			Template: &viewmodels.MessageTemplate{
				Template: dto.Template,
			},
			SaveURL:   fmt.Sprintf("%s/%d", c.basePath, id),
			DeleteURL: fmt.Sprintf("%s/%d", c.basePath, id),
		}
		templ.Handler(msgtui.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if _, err := c.templateService.Update(r.Context(), id, dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *MessageTemplateController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := c.templateService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
