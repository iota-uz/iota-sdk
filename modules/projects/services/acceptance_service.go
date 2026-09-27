package services

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
)

// ErrAcceptanceStatus means the document cannot move to the requested status:
// only a draft can be signed, and a cancelled document stays cancelled.
var ErrAcceptanceStatus = errors.New("acceptance document cannot change to this status")

type AcceptanceService struct {
	repo        acceptance.Repository
	projectRepo project.Repository
}

func NewAcceptanceService(repo acceptance.Repository, projectRepo project.Repository) *AcceptanceService {
	return &AcceptanceService{repo: repo, projectRepo: projectRepo}
}

func (s *AcceptanceService) GetByID(ctx context.Context, id uuid.UUID) (acceptance.Document, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *AcceptanceService) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]acceptance.Document, error) {
	return s.repo.GetByProjectID(ctx, projectID)
}

func (s *AcceptanceService) Create(ctx context.Context, document acceptance.Document) (acceptance.Document, error) {
	if _, err := s.projectRepo.GetByID(ctx, document.ProjectID()); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, document)
}

// Sign makes a draft document recognise its amount.
func (s *AcceptanceService) Sign(ctx context.Context, id uuid.UUID) (acceptance.Document, error) {
	return s.changeStatus(ctx, id, acceptance.StatusSigned, acceptance.StatusDraft)
}

// Cancel withdraws a draft or signed document, so it no longer counts.
func (s *AcceptanceService) Cancel(ctx context.Context, id uuid.UUID) (acceptance.Document, error) {
	return s.changeStatus(ctx, id, acceptance.StatusCancelled, acceptance.StatusDraft, acceptance.StatusSigned)
}

func (s *AcceptanceService) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

func (s *AcceptanceService) changeStatus(
	ctx context.Context, id uuid.UUID, status acceptance.Status, from ...acceptance.Status,
) (acceptance.Document, error) {
	document, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(from, document.Status()) {
		return nil, ErrAcceptanceStatus
	}
	return s.repo.Update(ctx, document.UpdateStatus(status))
}
