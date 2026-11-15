package controllers

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

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
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors/serrorhttp"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/sirupsen/logrus"
)

// AcceptanceController serves the acceptance tab of the project drawer and the
// drawer that creates, edits and deletes an acceptance document. Every tab
// action answers with the whole tab, so the revenue figures stay in step with
// the documents; the drawer answers by closing itself and telling the browser
// what to refresh (see AcceptanceSaveHook).
type AcceptanceController struct {
	basePath string
}

type acceptancePanel struct {
	documents *projectServices.AcceptanceService
	revenue   *projectServices.RevenueService
	forms     *projectServices.AcceptanceFormService
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
	router.HandleFunc("/drawer", di.H(c.NewDrawer)).Methods(http.MethodGet)
	router.HandleFunc("/drawer", di.H(c.SaveDrawer)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/drawer", di.H(c.EditDrawer)).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/drawer", di.H(c.SaveDrawer)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/drawer", di.H(c.DeleteDrawer)).Methods(http.MethodDelete)
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
	formService *projectServices.AcceptanceFormService,
) {
	panel := acceptancePanel{acceptanceService, revenueService, formService}
	projectID, err := uuid.Parse(mux.Vars(r)["projectId"])
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusBadRequest, nil)
		return
	}
	panel.render(w, r, logger, projectID)
}

// Create adds a document from the form fields and answers with the tab. The
// drawer is the form people use; this route stays for clients that post the
// fields directly, and answers a form that does not validate with 422.
func (c *AcceptanceController) Create(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	acceptanceService *projectServices.AcceptanceService,
	revenueService *projectServices.RevenueService,
	formService *projectServices.AcceptanceFormService,
) {
	panel := acceptancePanel{acceptanceService, revenueService, formService}
	projectID, err := uuid.Parse(mux.Vars(r)["projectId"])
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusBadRequest, nil)
		return
	}
	dto, err := composables.UseForm(&dtos.AcceptanceCreateDTO{}, r)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusBadRequest, nil)
		return
	}
	rules, ok := c.rules(w, r, logger, formService, projectID)
	if !ok {
		return
	}
	applyRules(dto, rules)
	if errorsMap, ok := dto.Check(r.Context(), kindValues(rules)); !ok {
		http.Error(w, joinErrors(errorsMap), http.StatusUnprocessableEntity)
		return
	}

	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}
	if _, err := acceptanceService.Create(r.Context(), dto.ToEntity(tenantID, projectID)); err != nil {
		logger.WithError(err).Error("Error creating acceptance document")
		http.Error(w, "Error creating acceptance document", acceptanceErrorStatus(err))
		return
	}
	panel.render(w, r, logger, projectID)
}

func (c *AcceptanceController) Sign(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	acceptanceService *projectServices.AcceptanceService,
	revenueService *projectServices.RevenueService,
	formService *projectServices.AcceptanceFormService,
) {
	panel := acceptancePanel{acceptanceService, revenueService, formService}
	c.change(w, r, logger, panel, acceptanceService.Sign)
}

func (c *AcceptanceController) Cancel(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	acceptanceService *projectServices.AcceptanceService,
	revenueService *projectServices.RevenueService,
	formService *projectServices.AcceptanceFormService,
) {
	panel := acceptancePanel{acceptanceService, revenueService, formService}
	c.change(w, r, logger, panel, acceptanceService.Cancel)
}

func (c *AcceptanceController) Delete(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	acceptanceService *projectServices.AcceptanceService,
	revenueService *projectServices.RevenueService,
	formService *projectServices.AcceptanceFormService,
) {
	panel := acceptancePanel{acceptanceService, revenueService, formService}
	projectID, id, ok := c.ids(w, r)
	if !ok {
		return
	}
	if err := acceptanceService.Delete(r.Context(), projectID, id); err != nil {
		logger.WithError(err).Error("Error deleting acceptance document")
		http.Error(w, "Error deleting acceptance document", acceptanceErrorStatus(err))
		return
	}
	panel.render(w, r, logger, projectID)
}

// NewDrawer opens the drawer of a new document, dated today and in the
// currency the project starts with.
func (c *AcceptanceController) NewDrawer(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	formService *projectServices.AcceptanceFormService,
	currencyService *coreservices.CurrencyService,
) {
	projectID, err := uuid.Parse(mux.Vars(r)["projectId"])
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusBadRequest, nil)
		return
	}
	rules, ok := c.rules(w, r, logger, formService, projectID)
	if !ok {
		return
	}
	c.renderDrawer(w, r, logger, currencyService, projectID, uuid.Nil, rules, blankForm(), map[string]string{})
}

// EditDrawer opens the drawer of a stored document.
func (c *AcceptanceController) EditDrawer(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	acceptanceService *projectServices.AcceptanceService,
	formService *projectServices.AcceptanceFormService,
	currencyService *coreservices.CurrencyService,
) {
	projectID, id, ok := c.ids(w, r)
	if !ok {
		return
	}
	document, err := acceptanceService.ProjectDocument(r.Context(), projectID, id)
	if err == nil && document.Status() == acceptance.StatusCancelled {
		err = acceptance.ErrStatus
	}
	if err != nil {
		logger.WithError(err).Error("Error retrieving acceptance document")
		http.Error(w, "Error retrieving acceptance document", acceptanceErrorStatus(err))
		return
	}
	rules, ok := c.rules(w, r, logger, formService, projectID)
	if !ok {
		return
	}
	c.renderDrawer(w, r, logger, currencyService, projectID, id, keepKind(rules, document.Kind()), mappers.AcceptanceDocumentToForm(document), map[string]string{})
}

// SaveDrawer creates a document, or saves the edited one when the route names
// it. A form that does not validate comes back in the drawer with the errors
// at its fields; a saved one closes the drawer and refreshes what the hook
// names.
func (c *AcceptanceController) SaveDrawer(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	acceptanceService *projectServices.AcceptanceService,
	formService *projectServices.AcceptanceFormService,
	currencyService *coreservices.CurrencyService,
	hook AcceptanceSaveHook,
) {
	projectID, err := uuid.Parse(mux.Vars(r)["projectId"])
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusBadRequest, nil)
		return
	}
	id := uuid.Nil
	if raw, editing := mux.Vars(r)["id"]; editing {
		if id, err = uuid.Parse(raw); err != nil {
			serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusBadRequest, nil)
			return
		}
	}
	dto, err := composables.UseForm(&dtos.AcceptanceCreateDTO{}, r)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusBadRequest, nil)
		return
	}
	rules, ok := c.rules(w, r, logger, formService, projectID)
	if !ok {
		return
	}
	if id != uuid.Nil {
		// A document keeps the kind it has even when the form no longer offers it.
		stored, err := acceptanceService.ProjectDocument(r.Context(), projectID, id)
		if err != nil {
			logger.WithError(err).Error("Error retrieving acceptance document")
			http.Error(w, "Error retrieving acceptance document", acceptanceErrorStatus(err))
			return
		}
		rules = keepKind(rules, stored.Kind())
	}
	applyRules(dto, rules)
	if errorsMap, ok := dto.Check(r.Context(), kindValues(rules)); !ok {
		c.renderDrawer(w, r, logger, currencyService, projectID, id, rules, *dto, errorsMap)
		return
	}

	var saved acceptance.Document
	outcome := AcceptanceChangeUpdated
	if id == uuid.Nil {
		outcome = AcceptanceChangeCreated
		tenantID, err := composables.UseTenantID(r.Context())
		if err != nil {
			serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
			return
		}
		saved, err = acceptanceService.Create(r.Context(), dto.ToEntity(tenantID, projectID))
		if err != nil {
			logger.WithError(err).Error("Error creating acceptance document")
			http.Error(w, "Error creating acceptance document", acceptanceErrorStatus(err))
			return
		}
	} else {
		saved, err = acceptanceService.Update(r.Context(), projectID, id, dto.ToRevision())
		if err != nil {
			logger.WithError(err).Error("Error updating acceptance document")
			http.Error(w, "Error updating acceptance document", acceptanceErrorStatus(err))
			return
		}
	}
	c.finish(w, r, hook, AcceptanceOutcome{
		Change: outcome, ProjectID: projectID, DocumentID: saved.ID(), Document: saved,
	})
}

// DeleteDrawer deletes the document of the open drawer.
func (c *AcceptanceController) DeleteDrawer(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	acceptanceService *projectServices.AcceptanceService,
	hook AcceptanceSaveHook,
) {
	projectID, id, ok := c.ids(w, r)
	if !ok {
		return
	}
	if err := acceptanceService.Delete(r.Context(), projectID, id); err != nil {
		logger.WithError(err).Error("Error deleting acceptance document")
		http.Error(w, "Error deleting acceptance document", acceptanceErrorStatus(err))
		return
	}
	c.finish(w, r, hook, AcceptanceOutcome{Change: AcceptanceChangeDeleted, ProjectID: projectID, DocumentID: id})
}

// finish answers a drawer request that succeeded: the empty body replaces the
// drawer, so it closes, and the reaction says what the page does next.
func (c *AcceptanceController) finish(w http.ResponseWriter, r *http.Request, hook AcceptanceSaveHook, outcome AcceptanceOutcome) {
	reaction := hook.Saved(r.Context(), outcome)
	if reaction.Redirect != "" {
		htmx.Redirect(w, reaction.Redirect)
		return
	}
	title := intl.TWithDefault(r.Context(), "Projects.Acceptance.Toasts."+string(outcome.Change), "")
	htmx.ToastSuccessAndRefresh(w, title, "", reaction.RefreshTargets...)
	w.WriteHeader(http.StatusOK)
}

func (c *AcceptanceController) change(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	panel acceptancePanel,
	action func(ctx context.Context, projectID, id uuid.UUID) (acceptance.Document, error),
) {
	projectID, id, ok := c.ids(w, r)
	if !ok {
		return
	}
	if _, err := action(r.Context(), projectID, id); err != nil {
		logger.WithError(err).Error("Error changing acceptance document status")
		http.Error(w, "Error changing acceptance document status", acceptanceErrorStatus(err))
		return
	}
	panel.render(w, r, logger, projectID)
}

// acceptanceErrorStatus answers a document of another project as not found and
// a status it cannot move to as a conflict.
func acceptanceErrorStatus(err error) int {
	switch {
	case errors.Is(err, acceptance.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, acceptance.ErrStatus):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func (c *AcceptanceController) ids(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	vars := mux.Vars(r)
	projectID, err := uuid.Parse(vars["projectId"])
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusBadRequest, nil)
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusBadRequest, nil)
		return uuid.Nil, uuid.Nil, false
	}
	return projectID, id, true
}

// blankForm is the form of a new document, dated today.
func blankForm() dtos.AcceptanceCreateDTO {
	return dtos.AcceptanceCreateDTO{Date: shared.DateOnly(time.Now())}
}

// applyRules fills what the project decides: the starting currency, and the
// currency itself when the project locks it.
func applyRules(dto *dtos.AcceptanceCreateDTO, rules projectServices.AcceptanceRules) {
	if rules.Currency.Locked || dto.CurrencyCode == "" {
		dto.CurrencyCode = rules.Currency.Code
	}
	if dto.Kind == "" && len(rules.Kinds) > 0 {
		dto.Kind = string(rules.Kinds[0].Value)
	}
}

// keepKind adds the kind of a stored document to the kinds the form offers, so
// editing the document does not silently change its kind.
func keepKind(rules projectServices.AcceptanceRules, kind acceptance.Kind) projectServices.AcceptanceRules {
	if rules.AllowsKind(kind) {
		return rules
	}
	kinds := append(append([]acceptance.KindOption{}, rules.Kinds...), acceptance.KindOption{Value: kind})
	rules.Kinds = kinds
	return rules
}

// rules reads what the form of the project allows; it has answered when it
// returns false.
func (c *AcceptanceController) rules(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	formService *projectServices.AcceptanceFormService,
	projectID uuid.UUID,
) (projectServices.AcceptanceRules, bool) {
	rules, err := formService.Rules(r.Context(), projectID)
	if err != nil {
		logger.WithError(err).Error("Error retrieving acceptance form rules")
		http.Error(w, "Error retrieving acceptance form rules", acceptanceErrorStatus(err))
		return projectServices.AcceptanceRules{}, false
	}
	return rules, true
}

func kindValues(rules projectServices.AcceptanceRules) []acceptance.Kind {
	kinds := make([]acceptance.Kind, 0, len(rules.Kinds))
	for _, option := range rules.Kinds {
		kinds = append(kinds, option.Value)
	}
	return kinds
}

func joinErrors(errorsMap map[string]string) string {
	fields := make([]string, 0, len(errorsMap))
	for field := range errorsMap {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	lines := make([]string, 0, len(fields))
	for _, field := range fields {
		lines = append(lines, errorsMap[field])
	}
	return strings.Join(lines, "\n")
}

// renderDrawer answers with the drawer of the document (a new one when id is
// nil), showing the form as it stands and the errors at its fields.
func (c *AcceptanceController) renderDrawer(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
	currencyService *coreservices.CurrencyService,
	projectID, id uuid.UUID,
	rules projectServices.AcceptanceRules,
	form dtos.AcceptanceCreateDTO,
	errorsMap map[string]string,
) {
	ctx := r.Context()
	applyRules(&form, rules)
	props := &projects.AcceptanceDrawerProps{
		ProjectID:      projectID.String(),
		Form:           form,
		Kinds:          rules.Kinds,
		CurrencyLocked: rules.Currency.Locked,
		Errors:         errorsMap,
	}
	if id != uuid.Nil {
		props.DocumentID = id.String()
	}
	if !rules.Currency.Locked {
		currencies, err := currencyService.GetAll(ctx)
		if err != nil {
			logger.WithError(err).Error("Error retrieving currencies")
			http.Error(w, "Error retrieving currencies", http.StatusInternalServerError)
			return
		}
		props.Currencies = mapping.MapViewModels(currencies, coremappers.CurrencyToViewModel)
	}
	templ.Handler(projects.AcceptanceDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

// render answers with the tab.
func (p acceptancePanel) render(w http.ResponseWriter, r *http.Request, logger *logrus.Entry, projectID uuid.UUID) {
	ctx := r.Context()
	rules, err := p.forms.Rules(ctx, projectID)
	if err != nil {
		logger.WithError(err).Error("Error retrieving acceptance form rules")
		http.Error(w, "Error retrieving acceptance form rules", http.StatusInternalServerError)
		return
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

	props := &projects.AcceptanceProps{
		ProjectID: projectID.String(),
		Revenue:   mapping.MapViewModels(revenue, financemappers.RevenueToViewModel),
		Documents: mapping.MapViewModels(documents, mappers.AcceptanceDocumentToViewModel),
		Kinds:     rules.Kinds,
	}
	templ.Handler(projects.AcceptancePanel(props), templ.WithStreaming()).ServeHTTP(w, r)
}
