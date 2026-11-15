package controllers

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
)

// AcceptanceChange says what the drawer did to the document.
type AcceptanceChange string

const (
	AcceptanceChangeCreated AcceptanceChange = "created"
	AcceptanceChangeUpdated AcceptanceChange = "updated"
	AcceptanceChangeDeleted AcceptanceChange = "deleted"
)

// AcceptanceOutcome describes a document the drawer has just saved or deleted.
type AcceptanceOutcome struct {
	Change     AcceptanceChange
	ProjectID  uuid.UUID
	DocumentID uuid.UUID
	// Document is the saved document; it is nil once the document is deleted.
	Document acceptance.Document
}

// AcceptanceReaction is what the browser does after the drawer has saved. The
// drawer itself always closes.
type AcceptanceReaction struct {
	// Redirect, when set, sends the browser to this address.
	Redirect string
	// RefreshTargets are CSS selectors of the regions to reload. They need
	// hx-get and hx-trigger="iota:refresh" to take part.
	RefreshTargets []string
}

// AcceptanceSaveHook lets the host application decide what refreshes once the
// acceptance drawer has saved or deleted a document. The project drawer's
// acceptance tab refreshes itself by default; an application that shows the
// documents on its own page provides its own hook.
type AcceptanceSaveHook interface {
	Saved(ctx context.Context, outcome AcceptanceOutcome) AcceptanceReaction
}

type refreshAcceptanceTab struct{}

// NewAcceptanceTabRefresh reloads the acceptance tab of the project drawer.
func NewAcceptanceTabRefresh() AcceptanceSaveHook {
	return refreshAcceptanceTab{}
}

func (refreshAcceptanceTab) Saved(_ context.Context, outcome AcceptanceOutcome) AcceptanceReaction {
	return AcceptanceReaction{RefreshTargets: []string{AcceptanceTabSelector(outcome.ProjectID)}}
}

// AcceptanceTabSelector selects the region of the project drawer that lists
// the acceptance documents of the project.
func AcceptanceTabSelector(projectID uuid.UUID) string {
	return fmt.Sprintf("#project-acceptance-%s", projectID)
}
