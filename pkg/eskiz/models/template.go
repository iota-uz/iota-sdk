package models

import eskizapi "github.com/iota-uz/eskiz"

// TemplateSubmission is the outcome of SubmitTemplate — the Eskiz-side
// template id the caller stores for later moderation-status polling and
// (eventually) for attributing sends to an approved template.
type TemplateSubmission interface {
	// Template is the echoed template body Eskiz has recorded. Usually
	// identical to what was submitted.
	Template() string
}

// NewTemplateSubmission wraps an Eskiz SendTemplateResponse.
func NewTemplateSubmission(resp *eskizapi.SendTemplateResponse) TemplateSubmission {
	if resp == nil {
		return nil
	}
	s := &templateSubmission{}
	if resp.Template != nil {
		s.template = *resp.Template
	}
	return s
}

type templateSubmission struct {
	template string
}

func (s *templateSubmission) Template() string { return s.template }

// TemplateModerationStatus is the moderation state Eskiz assigns to a
// previously-submitted template. It's derived from the /user/templates list
// endpoint — there is no single-template fetch.
//
// Canonical values per Eskiz OpenAPI spec (translated):
//   - "moderation" → На модерации (in moderation queue, treat as pending)
//   - "inproccess" → В процессе (review in flight, treat as pending)
//   - "service"    → Сервисный (approved for service messages)
//   - "reklama"    → Рекламный (approved for advertising)
//   - "rejected"   → Отказано (moderation denied)
//
// Consumers should treat unknown values as pending-equivalent.
type TemplateModerationStatus string

const (
	TemplateModerationPending     TemplateModerationStatus = "moderation"
	TemplateModerationInProcess   TemplateModerationStatus = "inproccess"
	TemplateModerationApproved    TemplateModerationStatus = "service"
	TemplateModerationAdvertising TemplateModerationStatus = "reklama"
	TemplateModerationRejected    TemplateModerationStatus = "rejected"
)

// TemplateRecord is a single entry in the user's template list.
type TemplateRecord interface {
	ID() int
	Template() string
	Status() TemplateModerationStatus
}

// NewTemplateRecords maps a TemplatesListResponse into domain records.
// Unknown status strings are returned verbatim for caller inspection.
func NewTemplateRecords(resp *eskizapi.TemplatesListResponse) []TemplateRecord {
	if resp == nil || resp.Result == nil {
		return nil
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
		if item.Status != nil {
			r.status = TemplateModerationStatus(*item.Status)
		}
		out = append(out, r)
	}
	return out
}

type templateRecord struct {
	id       int
	template string
	status   TemplateModerationStatus
}

func (r *templateRecord) ID() int                              { return r.id }
func (r *templateRecord) Template() string                     { return r.template }
func (r *templateRecord) Status() TemplateModerationStatus     { return r.status }
