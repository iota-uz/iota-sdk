package controllers

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	lensdocument "github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	lensserve "github.com/iota-uz/iota-sdk/pkg/lens/serve"
)

// FinanceDashboardSpec exposes the dashboard spec to the external tests.
func FinanceDashboardSpec(ctx context.Context, tenantID uuid.UUID) lens.DashboardSpec {
	return financeDashboardSpec(tenantID, financeText(ctx))
}

// FinanceOperationsSpec exposes the drill drawer spec for the given query.
func FinanceOperationsSpec(ctx context.Context, tenantID uuid.UUID, query url.Values) (lens.DashboardSpec, error) {
	drill, err := parseOperationsDrill(query)
	if err != nil {
		return lens.DashboardSpec{}, err
	}
	return financeOperationsSpec(tenantID, drill, financeText(ctx)), nil
}

const FinanceOperationsDocument = financeOperationsDocument

// NewFinanceDashboardTestController builds the dashboard controller over a
// given data source so tests can drive the drawer routes.
func NewFinanceDashboardTestController(ds datasource.DataSource) *DashboardController {
	return &DashboardController{ds: ds, runtime: runtime.New(runtime.Options{}), snapshots: lensdocument.NewMemoryStore(time.Minute, 16)}
}

// OperationsDocument serves the operations drawer document.
func (c *DashboardController) OperationsDocument(w http.ResponseWriter, r *http.Request) {
	c.operations(func(h *lensserve.Handlers) http.HandlerFunc { return h.Document })(w, r)
}

// OperationsExport serves the operations drawer export.
func (c *DashboardController) OperationsExport(w http.ResponseWriter, r *http.Request) {
	c.operations(func(h *lensserve.Handlers) http.HandlerFunc { return h.Export })(w, r)
}
