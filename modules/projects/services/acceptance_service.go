package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

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
func (s *AcceptanceService) Sign(ctx context.Context, projectID, id uuid.UUID) (acceptance.Document, error) {
	return s.changeStatus(ctx, projectID, id, acceptance.StatusSigned, acceptance.StatusDraft)
}

// Cancel withdraws a draft or signed document, so it no longer counts.
func (s *AcceptanceService) Cancel(ctx context.Context, projectID, id uuid.UUID) (acceptance.Document, error) {
	return s.changeStatus(ctx, projectID, id, acceptance.StatusCancelled, acceptance.StatusDraft, acceptance.StatusSigned)
}

func (s *AcceptanceService) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	if _, err := s.projectDocument(ctx, projectID, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// projectDocument finds the document only among the project's documents.
func (s *AcceptanceService) projectDocument(ctx context.Context, projectID, id uuid.UUID) (acceptance.Document, error) {
	const op serrors.Op = "AcceptanceService.projectDocument"
	document, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if document.ProjectID() != projectID {
		return nil, serrors.E(op, acceptance.ErrNotFound)
	}
	return document, nil
}

func (s *AcceptanceService) changeStatus(
	ctx context.Context, projectID, id uuid.UUID, status acceptance.Status, from ...acceptance.Status,
) (acceptance.Document, error) {
	document, err := s.projectDocument(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateStatus(ctx, document.UpdateStatus(status), from...)
}
