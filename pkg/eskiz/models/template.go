package models

import eskizapi "github.com/iota-uz/eskiz"

// TemplateSubmission is the outcome of SubmitTemplate. Eskiz's /user/template
// response carries only the echoed body — there is no upstream id at this
// stage. Callers that need the moderation id resolve it via ListTemplates.
type TemplateSubmission interface {
	Template() string
}

// NewTemplateSubmission always returns a non-nil TemplateSubmission so callers
// never receive a (nil result, nil error) pair from service.SubmitTemplate.
func NewTemplateSubmission(resp *eskizapi.SendTemplateResponse) TemplateSubmission {
	s := &templateSubmission{}
	if resp != nil && resp.Template != nil {
		s.template = *resp.Template
	}
	return s
}

type templateSubmission struct {
	template string
}

func (s *templateSubmission) Template() string { return s.template }

// TemplateModerationStatus mirrors Eskiz's status strings:
//   - "moderation"  → in moderation queue (pending)
//   - "inproccess"  → review in flight (pending)
//   - "service"     → approved for service messages
//   - "reklama"     → approved for advertising
//   - "rejected"    → moderation denied
//
// Unknown values flow through verbatim so callers can decide.
type TemplateModerationStatus string

const (
	TemplateModerationPending     TemplateModerationStatus = "moderation"
	TemplateModerationInProcess   TemplateModerationStatus = "inproccess"
	TemplateModerationApproved    TemplateModerationStatus = "service"
	TemplateModerationAdvertising TemplateModerationStatus = "reklama"
	TemplateModerationRejected    TemplateModerationStatus = "rejected"
)

// IsKnown reports whether the status matches a value Eskiz actually documents.
func (s TemplateModerationStatus) IsKnown() bool {
	switch s {
	case TemplateModerationPending,
		TemplateModerationInProcess,
		TemplateModerationApproved,
		TemplateModerationAdvertising,
		TemplateModerationRejected:
		return true
	}
	return false
}

// IsTerminal reports whether the status is a final moderation verdict
// (no further transitions expected without a re-submission).
func (s TemplateModerationStatus) IsTerminal() bool {
	switch s {
	case TemplateModerationApproved,
		TemplateModerationAdvertising,
		TemplateModerationRejected:
		return true
	case TemplateModerationPending, TemplateModerationInProcess:
		return false
	}
	return false
}

// TemplateRecord exposes both `template` (parsed/normalised body, blank for
// pending rows) and `original_text` (verbatim body Eskiz received). Match
// against OriginalText() — it's populated immediately on submit, while
// Template() is empty until moderation runs.
type TemplateRecord interface {
	ID() int
	Template() string
	OriginalText() string
	Status() TemplateModerationStatus
}

// NewTemplateRecords always returns a non-nil slice (empty on missing payload)
// so callers can iterate without nil checks.
func NewTemplateRecords(resp *eskizapi.TemplatesListResponse) []TemplateRecord {
	if resp == nil || resp.Result == nil {
		return []TemplateRecord{}
	}
	out := make([]TemplateRecord, 0, len(resp.Result))
	for _, item := range resp.Result {
		r := &templateRecord{}
		if item.Id != nil {
			r.id = *item.Id
		}
		if item.Template != nil {
			r.template = *item.Template
		}
		if item.OriginalText != nil {
			r.originalText = *item.OriginalText
		}
		if item.Status != nil {
			r.status = TemplateModerationStatus(*item.Status)
		}
		out = append(out, r)
	}
	return out
}

type templateRecord struct {
	id           int
	template     string
	originalText string
	status       TemplateModerationStatus
}

func (r *templateRecord) ID() int                          { return r.id }
func (r *templateRecord) Template() string                 { return r.template }
func (r *templateRecord) OriginalText() string             { return r.originalText }
func (r *templateRecord) Status() TemplateModerationStatus { return r.status }
