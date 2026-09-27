package services

import (
	"context"
	"sort"

	"github.com/google/uuid"
	financeservices "github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	projectstage "github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project_stage"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

// RevenueService keeps the bases of project revenue apart: the contract,
// signed acceptance documents, invoices and payments are summed separately
// per currency, and the part of the contract not yet accepted is
// contract minus accepted.
type RevenueService struct {
	projectRepo    project.Repository
	acceptanceRepo acceptance.Repository
	stageRepo      projectstage.Repository
	invoices       InvoiceSource
}

func NewRevenueService(
	projectRepo project.Repository,
	acceptanceRepo acceptance.Repository,
	stageRepo projectstage.Repository,
	invoices InvoiceSource,
) *RevenueService {
	return &RevenueService{
		projectRepo:    projectRepo,
		acceptanceRepo: acceptanceRepo,
		stageRepo:      stageRepo,
		invoices:       invoices,
	}
}

// NewClientRevenue gives finance the revenue of a client's projects.
func NewClientRevenue(revenue *RevenueService) financeservices.ClientRevenueSource {
	return revenue
}

func (s *RevenueService) ProjectRevenue(ctx context.Context, projectID uuid.UUID) ([]financeservices.Revenue, error) {
	p, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return s.revenue(ctx, []project.Project{p})
}

func (s *RevenueService) ClientRevenue(ctx context.Context, counterpartyID uuid.UUID) ([]financeservices.Revenue, error) {
	projects, err := s.projectRepo.GetByCounterpartyID(ctx, counterpartyID)
	if err != nil {
		return nil, err
	}
	return s.revenue(ctx, projects)
}

func (s *RevenueService) revenue(ctx context.Context, projects []project.Project) ([]financeservices.Revenue, error) {
	if len(projects) == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, 0, len(projects))
	contracts := make([]*money.Money, 0, len(projects))
	for _, p := range projects {
		ids = append(ids, p.ID())
		if p.Contract() != nil {
			contracts = append(contracts, p.Contract())
		}
	}

	accepted, err := s.acceptanceRepo.SignedTotals(ctx, ids)
	if err != nil {
		return nil, err
	}
	invoiced, err := s.invoices.Invoiced(ctx, ids)
	if err != nil {
		return nil, err
	}
	paid, err := s.stageRepo.PaidTotals(ctx, ids)
	if err != nil {
		return nil, err
	}

	byCurrency := map[string]*financeservices.Revenue{}
	line := func(amount *money.Money) *financeservices.Revenue {
		code := amount.Currency().Code
		if _, ok := byCurrency[code]; !ok {
			zero := money.New(0, code)
			byCurrency[code] = &financeservices.Revenue{Contract: zero, Accepted: zero, Open: zero, Invoiced: zero, Paid: zero}
		}
		return byCurrency[code]
	}
	for _, amount := range contracts {
		l := line(amount)
		l.Contract = money.New(l.Contract.Amount()+amount.Amount(), amount.Currency().Code)
	}
	for _, amount := range accepted {
		l := line(amount)
		l.Accepted = money.New(l.Accepted.Amount()+amount.Amount(), amount.Currency().Code)
	}
	for _, amount := range invoiced {
		l := line(amount)
		l.Invoiced = money.New(l.Invoiced.Amount()+amount.Amount(), amount.Currency().Code)
	}
	for _, amount := range paid {
		l := line(amount)
		l.Paid = money.New(l.Paid.Amount()+amount.Amount(), amount.Currency().Code)
	}

	codes := make([]string, 0, len(byCurrency))
	for code := range byCurrency {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	lines := make([]financeservices.Revenue, 0, len(codes))
	for _, code := range codes {
		l := byCurrency[code]
		l.Open = money.New(l.Contract.Amount()-l.Accepted.Amount(), code)
		lines = append(lines, *l)
	}
	return lines, nil
}
