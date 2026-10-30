package services

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	projectstage "github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project_stage"
)

const (
	ProjectResource      = "projects.project"
	ProjectStageResource = "projects.project_stage"
	AcceptanceResource   = "projects.acceptance"

	ProjectCreated      = "projects.project.created"
	ProjectUpdated      = "projects.project.updated"
	ProjectDeleted      = "projects.project.deleted"
	ProjectStageCreated = "projects.project_stage.created"
	ProjectStageUpdated = "projects.project_stage.updated"
	ProjectStageDeleted = "projects.project_stage.deleted"
	AcceptanceCreated   = "projects.acceptance.created"
	AcceptanceSigned    = "projects.acceptance.signed"
	AcceptanceCancelled = "projects.acceptance.cancelled"
	AcceptanceDeleted   = "projects.acceptance.deleted"
)

// HistoryDefinitions are the business history event types of the projects module.
func HistoryDefinitions() []history.Definition {
	defs := make([]history.Definition, 0, 10)
	defs = append(defs, history.Lifecycle("projects.project", "Projects.History", history.Resource{Name: ProjectResource, Fields: []history.Field{
		{Name: "name", LabelKey: "Projects.Single.Name", Type: history.ValueString},
		{Name: "counterparty_id", LabelKey: "Projects.Single.Counterparty", Type: history.ValueString},
		{Name: "description", LabelKey: "Projects.Single._Description", Type: history.ValueString},
		{Name: "contract", LabelKey: "Projects.Single.ContractAmount", Type: history.ValueMoney},
	}})...)
	defs = append(defs, history.Lifecycle("projects.project_stage", "ProjectStages.History", history.Resource{Name: ProjectStageResource, Fields: []history.Field{
		{Name: "project_id", LabelKey: "ProjectStages.Single.ProjectID", Type: history.ValueString},
		{Name: "stage_number", LabelKey: "ProjectStages.Single.StageNumber", Type: history.ValueInteger},
		{Name: "description", LabelKey: "ProjectStages.Single._Description", Type: history.ValueString},
		{Name: "total_amount", LabelKey: "ProjectStages.Single.TotalAmount", Type: history.ValueInteger},
		{Name: "start_date", LabelKey: "ProjectStages.Single.StartDate", Type: history.ValueDate},
		{Name: "planned_end_date", LabelKey: "ProjectStages.Single.PlannedEndDate", Type: history.ValueDate},
		{Name: "factual_end_date", LabelKey: "ProjectStages.Single.FactualEndDate", Type: history.ValueDate},
	}})...)
	acceptanceResource := history.Resource{Name: AcceptanceResource, Fields: []history.Field{
		{Name: "project_id", LabelKey: "ProjectStages.Single.ProjectID", Type: history.ValueString},
		{Name: "kind", LabelKey: "Projects.Acceptance.Kind", Type: history.ValueString},
		{Name: "number", LabelKey: "Projects.Acceptance.Number", Type: history.ValueString},
		{Name: "date", LabelKey: "Projects.Acceptance.Date", Type: history.ValueDate},
		{Name: "amount", LabelKey: "Projects.Acceptance.Amount", Type: history.ValueMoney},
		{Name: "status", LabelKey: "Projects.Acceptance.Status", Type: history.ValueString},
		{Name: "description", LabelKey: "Projects.Acceptance._Description", Type: history.ValueString},
	}}
	for _, a := range []struct{ eventType, key string }{
		{AcceptanceCreated, "Created"},
		{AcceptanceSigned, "Signed"},
		{AcceptanceCancelled, "Cancelled"},
		{AcceptanceDeleted, "Deleted"},
	} {
		defs = append(defs, history.Definition{
			Type:           a.eventType,
			Version:        1,
			Category:       businessevent.CategoryBusiness,
			TitleKey:       "Projects.Acceptance.History." + a.key + ".Title",
			DescriptionKey: "Projects.Acceptance.History." + a.key + ".Summary",
			Resources:      []history.Resource{acceptanceResource},
		})
	}
	return defs
}

func projectSnapshot(p project.Project) history.Snapshot {
	return history.Snapshot{
		"name":            p.Name(),
		"counterparty_id": history.UUID(p.CounterpartyID()),
		"description":     p.Description(),
		"contract":        history.Money(p.Contract()),
	}
}

func projectStageSnapshot(s projectstage.ProjectStage) history.Snapshot {
	return history.Snapshot{
		"project_id":       history.UUID(s.ProjectID()),
		"stage_number":     s.StageNumber(),
		"description":      s.Description(),
		"total_amount":     s.TotalAmount(),
		"start_date":       optionalDate(s.StartDate()),
		"planned_end_date": optionalDate(s.PlannedEndDate()),
		"factual_end_date": optionalDate(s.FactualEndDate()),
	}
}

func acceptanceSnapshot(d acceptance.Document) history.Snapshot {
	return history.Snapshot{
		"project_id":  history.UUID(d.ProjectID()),
		"kind":        string(d.Kind()),
		"number":      d.Number(),
		"date":        history.Date(d.Date()),
		"amount":      history.Money(d.Amount()),
		"status":      string(d.Status()),
		"description": d.Description(),
	}
}

func optionalDate(t *time.Time) any {
	if t == nil {
		return nil
	}
	return history.Date(*t)
}
