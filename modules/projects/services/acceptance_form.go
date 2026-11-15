package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// AcceptanceKinds lists the kinds of acceptance document the form offers.
// An application that signs other kinds than acts, delivery notes and
// protocols provides its own list; the first kind is the one a new document
// starts with.
type AcceptanceKinds interface {
	Kinds(ctx context.Context) []acceptance.KindOption
}

type defaultAcceptanceKinds struct{}

// NewDefaultAcceptanceKinds offers act, delivery note, protocol and other.
func NewDefaultAcceptanceKinds() AcceptanceKinds {
	return defaultAcceptanceKinds{}
}

func (defaultAcceptanceKinds) Kinds(context.Context) []acceptance.KindOption {
	return acceptance.DefaultKinds()
}

// AcceptanceCurrency is the currency a new document of a project starts with.
// Locked means the person cannot change it: every document of the project is
// in this currency, so that what is accepted never adds up unlike currencies.
type AcceptanceCurrency struct {
	Code   string
	Locked bool
}

// AcceptanceCurrencyPolicy decides the currency of a project's documents.
// An application whose projects have a billing currency of their own, for
// example the currency of the project invoices, provides the policy.
type AcceptanceCurrencyPolicy interface {
	ForProject(ctx context.Context, projectID uuid.UUID) (AcceptanceCurrency, error)
}

type contractCurrencyPolicy struct {
	projects project.Repository
}

// NewContractCurrencyPolicy starts a document in the currency of the project
// contract and leaves the choice to the person.
func NewContractCurrencyPolicy(projects project.Repository) AcceptanceCurrencyPolicy {
	return contractCurrencyPolicy{projects: projects}
}

func (p contractCurrencyPolicy) ForProject(ctx context.Context, projectID uuid.UUID) (AcceptanceCurrency, error) {
	const op serrors.Op = "contractCurrencyPolicy.ForProject"
	found, err := p.projects.GetByID(ctx, projectID)
	if err != nil {
		return AcceptanceCurrency{}, serrors.Wrap(op, err)
	}
	if contract := found.Contract(); contract != nil {
		return AcceptanceCurrency{Code: contract.Currency().Code}, nil
	}
	return AcceptanceCurrency{}, nil
}

// AcceptanceRules is what the form of one project allows.
type AcceptanceRules struct {
	Kinds    []acceptance.KindOption
	Currency AcceptanceCurrency
}

// AllowsKind reports whether the form offers the kind.
func (r AcceptanceRules) AllowsKind(kind acceptance.Kind) bool {
	for _, option := range r.Kinds {
		if option.Value == kind {
			return true
		}
	}
	return false
}

// AcceptanceFormService gathers the extension points of the acceptance form.
type AcceptanceFormService struct {
	kinds    AcceptanceKinds
	currency AcceptanceCurrencyPolicy
}

func NewAcceptanceFormService(kinds AcceptanceKinds, currency AcceptanceCurrencyPolicy) *AcceptanceFormService {
	return &AcceptanceFormService{kinds: kinds, currency: currency}
}

// Rules returns the kinds and the currency the form of the project allows.
func (s *AcceptanceFormService) Rules(ctx context.Context, projectID uuid.UUID) (AcceptanceRules, error) {
	const op serrors.Op = "AcceptanceFormService.Rules"
	kinds := make([]acceptance.KindOption, 0)
	for _, option := range s.kinds.Kinds(ctx) {
		if acceptance.ValidKind(option.Value) {
			kinds = append(kinds, option)
		}
	}
	if len(kinds) == 0 {
		kinds = acceptance.DefaultKinds()
	}
	currency, err := s.currency.ForProject(ctx, projectID)
	if err != nil {
		return AcceptanceRules{}, serrors.Wrap(op, err)
	}
	return AcceptanceRules{Kinds: kinds, Currency: currency}, nil
}
