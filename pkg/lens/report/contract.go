// Package report defines reusable analytical reports independently of storage and execution snapshots.
package report

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/filtercontext"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const ContractVersion = "1"

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// ErrIncompatible is the cause of the FailedPrecondition Resolve returns when a
// definition's source contract version or view is no longer offered by its source.
// Compose degrades such a placement instead of failing the composition.
var ErrIncompatible = errors.New("report definition is incompatible with its source")

type Parameters map[string]json.RawMessage

type SourceRef struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type Definition struct {
	Filters    filtercontext.ContextFilters `json:"filters,omitempty"`
	ID         string                       `json:"id"`
	Revision   string                       `json:"revision"`
	Title      string                       `json:"title"`
	Source     SourceRef                    `json:"source"`
	View       string                       `json:"view"`
	Parameters Parameters                   `json:"parameters,omitempty"`
}

type Placement struct {
	Filters    filtercontext.ContextFilters `json:"filters,omitempty"`
	SectionID  string                       `json:"sectionId,omitempty"`
	ID         string                       `json:"id"`
	ReportID   string                       `json:"reportId"`
	Title      *string                      `json:"title,omitempty"`
	Span       int                          `json:"span,omitempty"`
	Parameters Parameters                   `json:"parameters,omitempty"`
}

type Composition struct {
	Context    *filtercontext.ContextBoard `json:"context,omitempty"`
	ID         string                      `json:"id"`
	Revision   string                      `json:"revision"`
	Title      string                      `json:"title"`
	Parameters Parameters                  `json:"parameters,omitempty"`
	Rows       [][]Placement               `json:"rows"`
}

type Capabilities struct {
	OperationDimensions map[string][]string `json:"operationDimensions,omitempty"`
	Dimensions          []string            `json:"dimensions,omitempty"`
	ViewDimensions      map[string][]string `json:"viewDimensions,omitempty"`
	Filters             []string            `json:"filters"`
	Views               []string            `json:"views"`
	Drill               bool                `json:"drill"`
	Save                bool                `json:"save"`
	Export              bool                `json:"export"`
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
	// Status is VersionCurrent for an executed placement and VersionIncompatible
	// for one Compose rendered as unavailable without executing its source.
	Status     VersionStatus
	Context    *filtercontext.EffectiveContext
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
	if err := filtercontext.ValidateFilters(d.Filters, false); err != nil {
		return err
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
	if c.Context != nil {
		if err := filtercontext.ValidateBoard(*c.Context); err != nil {
			return err
		}
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
			if err := filtercontext.ValidateFilters(placement.Filters, false); err != nil {
				return err
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
	resolved, err := resolve(ctx, host, placement, defaults, nil)
	if err != nil {
		return Resolved{}, err
	}
	return resolved, nil
}

func incompatible(message string) error {
	return serrors.New(serrors.FailedPrecondition, message).WithCause(ErrIncompatible).WithOp("lens.report.Resolve")
}

func resolve(ctx context.Context, host Host, placement Placement, defaults Parameters, input *filtercontext.ContextInput) (Resolved, error) {
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
		return Resolved{Status: VersionIncompatible, Definition: definition, Placement: clone(placement), Source: descriptor}, incompatible("report source contract is incompatible")
	}
	if !slices.Contains(descriptor.Capabilities.Views, definition.View) {
		return Resolved{Status: VersionIncompatible, Definition: definition, Placement: clone(placement), Source: descriptor}, incompatible("report view is unavailable")
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
	var effective *filtercontext.EffectiveContext
	if input != nil {
		scoped := clone(*input)
		scoped.OperationDimensions = clone(descriptor.Capabilities.OperationDimensions)
		scoped.Source = &filtercontext.ContextSource{ID: descriptor.ID, Version: descriptor.Version, Revision: descriptor.Revision, View: definition.View}
		scoped.SupportedDimensions = descriptor.Capabilities.Dimensions
		if dimensions, exists := descriptor.Capabilities.ViewDimensions[definition.View]; exists {
			scoped.SupportedDimensions = dimensions
		}
		supportsPeriod := slices.Contains(descriptor.Capabilities.Filters, "period")
		scoped.SupportsPeriod = &supportsPeriod
		scoped.Layers = append([]filtercontext.ContextLayer{{Origin: filtercontext.ContextOrigin{Scope: "report", ID: definition.ID}, Filters: definition.Filters}}, scoped.Layers...)
		value, contextErr := filtercontext.Resolve(scoped)
		if contextErr != nil {
			return Resolved{}, contextErr
		}
		effective = &value
	}
	var spec lens.DashboardSpec
	if effective != nil && !effective.Ready() {
		data, frameErr := frame.NewBuilder("context-blocked").String("reason", frame.RoleDimension).FrameSet()
		if frameErr != nil {
			return Resolved{}, frameErr
		}
		spec = lens.DashboardSpec{ID: definition.ID, Datasets: []lens.DatasetSpec{{Name: "context-blocked", Kind: lens.DatasetKindStatic, Static: data}}, Rows: []lens.RowSpec{{Panels: []panel.Spec{panel.Table("context-blocked", "", "context-blocked").IDField("reason").Columns(panel.TableColumn{Field: "reason", Label: "Context"}).Terminal().Build()}}}}
	} else if effective != nil {
		scopedSource, ok := source.(ContextSource)
		if !ok {
			return Resolved{}, invalid("source must explicitly implement contextual execution")
		}
		spec, err = scopedSource.BuildContext(ctx, clone(definition), clone(parameters), clone(*effective))
	} else {
		spec, err = source.Build(ctx, clone(definition), clone(parameters))
	}
	if err != nil {
		return Resolved{}, serrors.Wrap("lens.report.Build", err)
	}
	return Resolved{Status: VersionCurrent, Context: effective, Definition: definition, Placement: clone(placement), Source: descriptor, Parameters: parameters, Spec: clone(spec)}, nil
}

// VersionStatus compares an opened result with the host's latest definition and source.
type VersionStatus string

const (
	// VersionCurrent means the opened result still describes the latest definition and source.
	VersionCurrent VersionStatus = "current"
	// VersionUpdateAvailable means reopening resolves a newer compatible definition or source.
	VersionUpdateAvailable VersionStatus = "update-available"
	// VersionIncompatible means the latest definition cannot run on the current source contract or view.
	VersionIncompatible VersionStatus = "incompatible"
)

// Changed reports whether the host holds a newer definition or source than the opened result.
func (r Resolved) Changed(ctx context.Context, host Host) (bool, error) {
	status, err := r.Version(ctx, host)
	if err != nil {
		return false, err
	}
	return status != VersionCurrent, nil
}

// Version classifies the host's latest definition and source against the opened result.
// The opened result is never mutated; the caller decides whether to offer an update.
func (r Resolved) Version(ctx context.Context, host Host) (VersionStatus, error) {
	if host == nil {
		return "", invalid("report host is required")
	}
	latest, err := host.Report(ctx, r.Definition.ID)
	if err != nil {
		return "", serrors.Wrap("lens.report.Version", err)
	}
	source, err := host.Source(ctx, latest.Source.ID)
	if err != nil {
		return "", serrors.Wrap("lens.report.Version", err)
	}
	if source == nil {
		return "", invalid("host returned an absent source")
	}
	return CompareVersion(r.Definition, r.Source, latest, source.Descriptor()), nil
}

// CompareVersion is the storage-agnostic rule behind Resolved.Version. Hosts that already
// hold both revisions, for example a client polling for updates, use it directly.
func CompareVersion(opened Definition, openedSource SourceDescriptor, latest Definition, latestSource SourceDescriptor) VersionStatus {
	if latest.Source.ID != latestSource.ID || latest.Source.Version != latestSource.Version || !slices.Contains(latestSource.Capabilities.Views, latest.View) {
		return VersionIncompatible
	}
	if latest.Revision != opened.Revision || latest.Source != opened.Source || latestSource.Revision != openedSource.Revision || latestSource.Version != openedSource.Version {
		return VersionUpdateAvailable
	}
	return VersionCurrent
}

type ContextSource interface {
	BuildContext(context.Context, Definition, Parameters, filtercontext.EffectiveContext) (lens.DashboardSpec, error)
}
