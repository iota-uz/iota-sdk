package controllers

import (
	"context"
	"net/url"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/lens"
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
