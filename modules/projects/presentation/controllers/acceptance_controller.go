package controllers

import (
	"context"
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	coremappers "github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	financemappers "github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/templates/pages/projects"
	projectServices "github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/sirupsen/logrus"
)

// AcceptanceController serves the acceptance tab of the project drawer. Every
// action answers with the whole tab, so the revenue figures stay in step with
// the documents.
type AcceptanceController struct {
	basePath string
}

type acceptancePanel struct {
	documents  *projectServices.AcceptanceService
	revenue    *projectServices.RevenueService
	currencies *coreservices.CurrencyService
	projects   *projectServices.ProjectService
}

func NewAcceptanceController() application.Controller {
	return &AcceptanceController{basePath: "/projects/{projectId:[0-9a-fA-F-]+}/acceptance"}
}

func (c *AcceptanceController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("projects.acceptance", 0, application.Route("", c.basePath))
}

func (c *AcceptanceController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", di.H(c.Panel)).Methods(http.MethodGet)
	router.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/sign", di.H(c.Sign)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/cancel", di.H(c.Cancel)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", di.H(c.Delete)).Methods(http.MethodDelete)
}

func (c *AcceptanceController) Panel(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	acceptanceService *projectServices.AcceptanceService,
	revenueService *projectServices.RevenueService,
	currencyService *coreservices.CurrencyService,
	projectService *projectServices.ProjectService,
) {
	panel := acceptancePanel{acceptanceService, revenueService, currencyService, projectService}
	projectID, err := uuid.Parse(mux.Vars(r)["projectId"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	panel.render(w, r, logger, projectID, dtos.AcceptanceCreateDTO{}, map[string]string{})
}

func (c *AcceptanceController) Create(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	acceptanceService *projectServices.AcceptanceService,
	revenueService *projectServices.RevenueService,
	currencyService *coreservices.CurrencyService,
	projectService *projectServices.ProjectService,
) {
	panel := acceptancePanel{acceptanceService, revenueService, currencyService, projectService}
	projectID, err := uuid.Parse(mux.Vars(r)["projectId"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dto, err := composables.UseForm(&dtos.AcceptanceCreateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		panel.render(w, r, logger, projectID, *dto, errorsMap)
		return
	}

	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := acceptanceService.Create(r.Context(), dto.ToEntity(tenantID, projectID)); err != nil {
		logger.WithError(err).Error("Error creating acceptance document")
		http.Error(w, "Error creating acceptance document", http.StatusInternalServerError)
		return
	}
	panel.render(w, r, logger, projectID, dtos.AcceptanceCreateDTO{}, map[string]string{})
}

func (c *AcceptanceController) Sign(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	acceptanceService *projectServices.AcceptanceService,
	revenueService *projectServices.RevenueService,
	currencyService *coreservices.CurrencyService,
	projectService *projectServices.ProjectService,
) {
	panel := acceptancePanel{acceptanceService, revenueService, currencyService, projectService}
	c.change(w, r, logger, panel, acceptanceService.Sign)
}

func (c *AcceptanceController) Cancel(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	acceptanceService *projectServices.AcceptanceService,
	revenueService *projectServices.RevenueService,
	currencyService *coreservices.CurrencyService,
	projectService *projectServices.ProjectService,
) {
	panel := acceptancePanel{acceptanceService, revenueService, currencyService, projectService}
	c.change(w, r, logger, panel, acceptanceService.Cancel)
}

func (c *AcceptanceController) Delete(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	acceptanceService *projectServices.AcceptanceService,
	revenueService *projectServices.RevenueService,
	currencyService *coreservices.CurrencyService,
	projectService *projectServices.ProjectService,
) {
	panel := acceptancePanel{acceptanceService, revenueService, currencyService, projectService}
	projectID, id, ok := c.ids(w, r)
	if !ok {
		return
	}
	if err := acceptanceService.Delete(r.Context(), id); err != nil {
		logger.WithError(err).Error("Error deleting acceptance document")
		http.Error(w, "Error deleting acceptance document", http.StatusInternalServerError)
		return
	}
	panel.render(w, r, logger, projectID, dtos.AcceptanceCreateDTO{}, map[string]string{})
}

func (c *AcceptanceController) change(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	panel acceptancePanel,
	action func(ctx context.Context, id uuid.UUID) (acceptance.Document, error),
) {
	projectID, id, ok := c.ids(w, r)
	if !ok {
		return
	}
	if _, err := action(r.Context(), id); err != nil {
		if errors.Is(err, projectServices.ErrAcceptanceStatus) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		logger.WithError(err).Error("Error changing acceptance document status")
		http.Error(w, "Error changing acceptance document status", http.StatusInternalServerError)
		return
	}
	panel.render(w, r, logger, projectID, dtos.AcceptanceCreateDTO{}, map[string]string{})
}

func (c *AcceptanceController) ids(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	vars := mux.Vars(r)
	projectID, err := uuid.Parse(vars["projectId"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return uuid.Nil, uuid.Nil, false
	}
	return projectID, id, true
}

// render answers with the tab; an empty form currency defaults to the
// contract currency.
func (p acceptancePanel) render(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	projectID uuid.UUID,
	form dtos.AcceptanceCreateDTO,
	errorsMap map[string]string,
) {
	ctx := r.Context()
	if form.CurrencyCode == "" {
		project, err := p.projects.GetByID(ctx, projectID)
		if err != nil {
			logger.WithError(err).Error("Error retrieving project")
			http.Error(w, "Error retrieving project", http.StatusInternalServerError)
			return
		}
		if contract := project.Contract(); contract != nil {
			form.CurrencyCode = contract.Currency().Code
		}
	}
	documents, err := p.documents.GetByProjectID(ctx, projectID)
	if err != nil {
		logger.WithError(err).Error("Error retrieving acceptance documents")
		http.Error(w, "Error retrieving acceptance documents", http.StatusInternalServerError)
		return
	}
	revenue, err := p.revenue.ProjectRevenue(ctx, projectID)
	if err != nil {
		logger.WithError(err).Error("Error retrieving project revenue")
		http.Error(w, "Error retrieving project revenue", http.StatusInternalServerError)
		return
	}
	currencies, err := p.currencies.GetAll(ctx)
	if err != nil {
		logger.WithError(err).Error("Error retrieving currencies")
		http.Error(w, "Error retrieving currencies", http.StatusInternalServerError)
		return
	}

	props := &projects.AcceptanceProps{
		ProjectID:  projectID.String(),
		Revenue:    mapping.MapViewModels(revenue, financemappers.RevenueToViewModel),
		Documents:  mapping.MapViewModels(documents, mappers.AcceptanceDocumentToViewModel),
		Form:       form,
		Currencies: mapping.MapViewModels(currencies, coremappers.CurrencyToViewModel),
		Errors:     errorsMap,
	}
	templ.Handler(projects.AcceptancePanel(props), templ.WithStreaming()).ServeHTTP(w, r)
}
