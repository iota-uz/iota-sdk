package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type AcceptanceService struct {
	repo          acceptance.Repository
	projectRepo   project.Repository
	historyWriter history.Writer
}

func NewAcceptanceService(repo acceptance.Repository, projectRepo project.Repository, historyWriter history.Writer) *AcceptanceService {
	return &AcceptanceService{repo: repo, projectRepo: projectRepo, historyWriter: historyWriter}
}

func (s *AcceptanceService) GetByID(ctx context.Context, id uuid.UUID) (acceptance.Document, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *AcceptanceService) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]acceptance.Document, error) {
	return s.repo.GetByProjectID(ctx, projectID)
}

func (s *AcceptanceService) Create(ctx context.Context, document acceptance.Document) (acceptance.Document, error) {
	var created acceptance.Document
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		if _, err := s.projectRepo.GetByID(txCtx, document.ProjectID()); err != nil {
			return err
		}
		var err error
		created, err = s.repo.Create(txCtx, document)
		if err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: AcceptanceCreated, Version: 1, Resource: AcceptanceResource, ID: created.ID().String(),
			After: acceptanceSnapshot(created),
		})
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// Sign makes a draft document recognise its amount.
func (s *AcceptanceService) Sign(ctx context.Context, projectID, id uuid.UUID) (acceptance.Document, error) {
	return s.changeStatus(ctx, projectID, id, AcceptanceSigned, acceptance.StatusSigned, acceptance.StatusDraft)
}

// Cancel withdraws a draft or signed document, so it no longer counts.
func (s *AcceptanceService) Cancel(ctx context.Context, projectID, id uuid.UUID) (acceptance.Document, error) {
	return s.changeStatus(ctx, projectID, id, AcceptanceCancelled, acceptance.StatusCancelled, acceptance.StatusDraft, acceptance.StatusSigned)
}

func (s *AcceptanceService) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	return composables.InTx(ctx, func(txCtx context.Context) error {
		document, err := s.projectDocument(txCtx, projectID, id)
		if err != nil {
			return err
		}
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: AcceptanceDeleted, Version: 1, Resource: AcceptanceResource, ID: id.String(),
			Before: acceptanceSnapshot(document),
		})
	})
}

// projectDocument finds the document only among the project's documents.
func (s *AcceptanceService) projectDocument(ctx context.Context, projectID, id uuid.UUID) (acceptance.Document, error) {
	const op serrors.Op = "AcceptanceService.projectDocument"
	document, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if document.ProjectID() != projectID {
		return nil, serrors.Wrap(op, acceptance.ErrNotFound)
	}
	return document, nil
}

func (s *AcceptanceService) changeStatus(
	ctx context.Context, projectID, id uuid.UUID, eventType string, status acceptance.Status, from ...acceptance.Status,
) (acceptance.Document, error) {
	var updated acceptance.Document
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		document, err := s.projectDocument(txCtx, projectID, id)
		if err != nil {
			return err
		}
		updated, err = s.repo.UpdateStatus(txCtx, document.UpdateStatus(status), from...)
		if err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: eventType, Version: 1, Resource: AcceptanceResource, ID: id.String(),
			Before: acceptanceSnapshot(document), After: acceptanceSnapshot(updated),
		})
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}
