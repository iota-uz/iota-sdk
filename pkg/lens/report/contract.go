// Package report defines reusable analytical reports independently of storage and execution snapshots.
package report

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const ContractVersion = "1"

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

type Parameters map[string]json.RawMessage

type SourceRef struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type Definition struct {
	ID         string     `json:"id"`
	Revision   string     `json:"revision"`
	Title      string     `json:"title"`
	Source     SourceRef  `json:"source"`
	View       string     `json:"view"`
	Parameters Parameters `json:"parameters,omitempty"`
}

type Placement struct {
	ID         string     `json:"id"`
	ReportID   string     `json:"reportId"`
	Title      *string    `json:"title,omitempty"`
	Span       int        `json:"span,omitempty"`
	Parameters Parameters `json:"parameters,omitempty"`
}

type Composition struct {
	ID         string        `json:"id"`
	Revision   string        `json:"revision"`
	Title      string        `json:"title"`
	Parameters Parameters    `json:"parameters,omitempty"`
	Rows       [][]Placement `json:"rows"`
}

type Capabilities struct {
	Filters []string `json:"filters"`
	Views   []string `json:"views"`
	Drill   bool     `json:"drill"`
	Save    bool     `json:"save"`
	Export  bool     `json:"export"`
}

type SourceDescriptor struct {
	ID           string       `json:"id"`
	Version      int          `json:"version"`
	Revision     string       `json:"revision"`
	Capabilities Capabilities `json:"capabilities"`
}

// Host applies authorization when resolving both definitions and sources.
type Host interface {
	Report(context.Context, string) (Definition, error)
	Source(context.Context, string) (Source, error)
}

type Source interface {
	Descriptor() SourceDescriptor
	Build(context.Context, Definition, Parameters) (lens.DashboardSpec, error)
}

type Resolved struct {
	Definition Definition
	Placement  Placement
	Source     SourceDescriptor
	Parameters Parameters
	Spec       lens.DashboardSpec
}

func invalid(message string) error {
	return serrors.New(serrors.Invalid, message).WithOp("lens.report")
}

func (d Definition) Validate() error {
	if !identifier.MatchString(d.ID) || strings.TrimSpace(d.Title) == "" || d.Revision == "" {
		return invalid("report identity, revision and title are required")
	}
	if !identifier.MatchString(d.Source.ID) || d.Source.Version < 1 || d.View == "" {
		return invalid("report source version and view are required")
	}
	return validateParameters(d.Parameters)
}

func (c Composition) Validate() error {
	if !identifier.MatchString(c.ID) || c.Revision == "" || strings.TrimSpace(c.Title) == "" {
		return invalid("composition identity, revision and title are required")
	}
	if err := validateParameters(c.Parameters); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, row := range c.Rows {
		if len(row) == 0 {
			return invalid("composition rows cannot be empty")
		}
		total := 0
		for _, placement := range row {
			if !identifier.MatchString(placement.ID) || !identifier.MatchString(placement.ReportID) || seen[placement.ID] {
				return invalid("placements require unique identity and report reference")
			}
			seen[placement.ID] = true
			span := placement.Span
			if span == 0 {
				span = 12
			}
			if span < 1 || span > 12 {
				return invalid("placement span must be between 1 and 12")
			}
			total += span
			if placement.Title != nil && strings.TrimSpace(*placement.Title) == "" {
				return invalid("placement title cannot be empty")
			}
			if err := validateParameters(placement.Parameters); err != nil {
				return err
			}
		}
		if total > 12 {
			return invalid("composition row exceeds twelve columns")
		}
	}
	return nil
}

func validateParameters(params Parameters) error {
	for key, value := range params {
		if !identifier.MatchString(key) || !json.Valid(value) {
			return invalid("invalid report parameter")
		}
	}
	return nil
}

func DecodeDefinition(data []byte) (Definition, error) {
	var definition Definition
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&definition); err != nil {
		return Definition{}, serrors.New(serrors.Invalid, "invalid report definition").WithCause(err).WithOp("lens.report.Decode")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Definition{}, invalid("unexpected trailing definition data")
	}
	return definition, definition.Validate()
}

func CopyDefinition(definition Definition, id, revision string) (Definition, error) {
	copied := clone(definition)
	if id == definition.ID {
		return Definition{}, invalid("a copied report needs a new identity")
	}
	copied.ID = id
	copied.Revision = revision
	return copied, copied.Validate()
}

func DuplicatePlacement(placement Placement, id string) (Placement, error) {
	if !identifier.MatchString(id) || id == placement.ID {
		return Placement{}, invalid("a duplicated placement needs a new identity")
	}
	copied := clone(placement)
	copied.ID = id
	return copied, nil
}

func Resolve(ctx context.Context, host Host, placement Placement, defaults Parameters) (Resolved, error) {
	if host == nil {
		return Resolved{}, invalid("report host is required")
	}
	if !identifier.MatchString(placement.ID) || !identifier.MatchString(placement.ReportID) {
		return Resolved{}, invalid("invalid placement reference")
	}
	if err := validateParameters(defaults); err != nil {
		return Resolved{}, err
	}
	if err := validateParameters(placement.Parameters); err != nil {
		return Resolved{}, err
	}
	definition, err := host.Report(ctx, placement.ReportID)
	if err != nil {
		return Resolved{}, serrors.Wrap("lens.report.Resolve", err)
	}
	definition = clone(definition)
	if err = definition.Validate(); err != nil {
		return Resolved{}, err
	}
	if definition.ID != placement.ReportID {
		return Resolved{}, invalid("host returned a different report")
	}
	source, err := host.Source(ctx, definition.Source.ID)
	if err != nil {
		return Resolved{}, serrors.Wrap("lens.report.Resolve", err)
	}
	if source == nil {
		return Resolved{}, invalid("host returned an absent source")
	}
	descriptor := clone(source.Descriptor())
	if descriptor.ID != definition.Source.ID || descriptor.Revision == "" {
		return Resolved{}, invalid("invalid source descriptor")
	}
	if descriptor.Version != definition.Source.Version {
		return Resolved{}, serrors.New(serrors.FailedPrecondition, "report source contract is incompatible").WithOp("lens.report.Resolve")
	}
	if !slices.Contains(descriptor.Capabilities.Views, definition.View) {
		return Resolved{}, serrors.New(serrors.FailedPrecondition, "report view is unavailable")
	}
	parameters := clone(definition.Parameters)
	if parameters == nil {
		parameters = Parameters{}
	}
	for _, layer := range []Parameters{defaults, placement.Parameters} {
		for key, value := range layer {
			parameters[key] = slices.Clone(value)
		}
	}
	for key := range parameters {
		if !slices.Contains(descriptor.Capabilities.Filters, key) {
			return Resolved{}, serrors.New(serrors.FailedPrecondition, "report parameter is unsupported")
		}
	}
	spec, err := source.Build(ctx, clone(definition), clone(parameters))
	if err != nil {
		return Resolved{}, serrors.Wrap("lens.report.Build", err)
	}
	return Resolved{Definition: definition, Placement: clone(placement), Source: descriptor, Parameters: parameters, Spec: clone(spec)}, nil
}

func (r Resolved) Changed(ctx context.Context, host Host) (bool, error) {
	latest, err := host.Report(ctx, r.Definition.ID)
	if err != nil {
		return false, err
	}
	source, err := host.Source(ctx, latest.Source.ID)
	if err != nil {
		return false, err
	}
	if source == nil {
		return false, invalid("host returned an absent source")
	}
	descriptor := source.Descriptor()
	return latest.Revision != r.Definition.Revision || latest.Source != r.Definition.Source || descriptor.Revision != r.Source.Revision || descriptor.Version != r.Source.Version, nil
}
