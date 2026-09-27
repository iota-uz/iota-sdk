package document

import (
	"fmt"
	"math"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
)

// TemporalSeries names one server-computed overlay column.
type TemporalSeries struct {
	Field string `json:"field"`
	Label string `json:"label,omitempty"`
}

// TemporalMovingAverage is a selectable SMA overlay.
type TemporalMovingAverage struct {
	Window int    `json:"window"`
	Field  string `json:"field"`
	Label  string `json:"label,omitempty"`
}

type TemporalPeriodState string

const (
	TemporalPeriodYTD        TemporalPeriodState = "ytd"
	TemporalPeriodAnnualized TemporalPeriodState = "annualized"
)

// TemporalPeriod marks one incomplete category. AnnualizedField, when set,
// is a distinct estimate and never replaces the observed value.
type TemporalPeriod struct {
	Category        string              `json:"category"`
	State           TemporalPeriodState `json:"state"`
	Label           string              `json:"label,omitempty"`
	AnnualizedField string              `json:"annualizedField,omitempty"`
}

// TemporalAnnotation is a labelled vertical event marker.
type TemporalAnnotation struct {
	At    string `json:"at"`
	Label string `json:"label"`
}

// TemporalForecast names projected values and their lower/upper confidence
// bounds. Start visually separates the projection from observed history.
type TemporalForecast struct {
	Start      string `json:"start"`
	ValueField string `json:"valueField"`
	LowerField string `json:"lowerField"`
	UpperField string `json:"upperField"`
	Label      string `json:"label,omitempty"`
}

// PanelTemporal is the generic time-series readability contract. Every
// computed overlay references a frame field so exports and charts agree.
type PanelTemporal struct {
	Regression     *TemporalSeries         `json:"regression,omitempty"`
	MovingAverages []TemporalMovingAverage `json:"movingAverages,omitempty"`
	ReferenceLines []PanelTarget           `json:"referenceLines,omitempty"`
	Period         *TemporalPeriod         `json:"period,omitempty"`
	Annotations    []TemporalAnnotation    `json:"annotations,omitempty"`
	Forecast       *TemporalForecast       `json:"forecast,omitempty"`
}

func buildTemporal(spec panel.Spec) *PanelTemporal {
	if spec.Temporal == nil && (spec.Kind != panel.KindTimeSeries || spec.Target == nil) {
		return nil
	}
	out := &PanelTemporal{}
	if temporal := spec.Temporal; temporal != nil {
		if !temporal.RegressionField.Empty() {
			out.Regression = &TemporalSeries{Field: temporal.RegressionField.Name(), Label: temporal.RegressionLabel}
		}
		for _, average := range temporal.MovingAverages {
			out.MovingAverages = append(out.MovingAverages, TemporalMovingAverage{
				Window: average.Window, Field: average.Field.Name(), Label: average.Label,
			})
		}
		for _, reference := range temporal.ReferenceLines {
			out.ReferenceLines = append(out.ReferenceLines, PanelTarget{Value: reference.Value, Label: reference.Label})
		}
		if temporal.Period != nil {
			out.Period = &TemporalPeriod{
				Category:        temporal.Period.Category,
				State:           TemporalPeriodState(temporal.Period.State),
				Label:           temporal.Period.Label,
				AnnualizedField: temporal.Period.AnnualizedField.Name(),
			}
		}
		for _, annotation := range temporal.Annotations {
			out.Annotations = append(out.Annotations, TemporalAnnotation{At: annotation.At, Label: annotation.Label})
		}
		if temporal.Forecast != nil {
			out.Forecast = &TemporalForecast{
				Start:      temporal.Forecast.Start,
				ValueField: temporal.Forecast.ValueField.Name(),
				LowerField: temporal.Forecast.LowerField.Name(),
				UpperField: temporal.Forecast.UpperField.Name(),
				Label:      temporal.Forecast.Label,
			}
		}
	}
	// TargetSpec predates temporal overlays. On a time series it is the first
	// horizontal reference line, preserving that public API while allowing
	// newer panels to declare additional references.
	if spec.Kind == panel.KindTimeSeries && spec.Target != nil {
		out.ReferenceLines = append([]PanelTarget{{Value: spec.Target.Value, Label: spec.Target.Label}}, out.ReferenceLines...)
	}
	return out
}

func validateTemporalPanel(panel Panel, frame Frame) error {
	temporal := panel.Temporal
	if temporal == nil {
		return nil
	}
	lineLike := panel.Kind == PanelKindLine || panel.Kind == PanelKindArea
	barPeriodOnly := panel.Kind == PanelKindBar && temporal.Period != nil &&
		temporal.Regression == nil && len(temporal.MovingAverages) == 0 &&
		len(temporal.ReferenceLines) == 0 && len(temporal.Annotations) == 0 && temporal.Forecast == nil
	if !lineLike && !barPeriodOnly {
		return fmt.Errorf("panel %s temporal overlays are only supported on line and area kinds, got %q", panel.ID, panel.Kind)
	}
	requireNumber := func(role, field string) error {
		if strings.TrimSpace(field) == "" {
			return fmt.Errorf("panel %s %s field is required", panel.ID, role)
		}
		if panel.Deferred {
			return nil
		}
		return requireNumberFrameColumn("panel "+panel.ID, role, frame, field)
	}
	if temporal.Regression != nil {
		if err := requireNumber("regression", temporal.Regression.Field); err != nil {
			return err
		}
	}
	windows := make(map[int]struct{}, len(temporal.MovingAverages))
	for _, average := range temporal.MovingAverages {
		switch average.Window {
		case 3, 7, 12:
		default:
			return fmt.Errorf("panel %s moving-average window must be 3, 7, or 12, got %d", panel.ID, average.Window)
		}
		if _, duplicate := windows[average.Window]; duplicate {
			return fmt.Errorf("panel %s has duplicate moving-average window %d", panel.ID, average.Window)
		}
		windows[average.Window] = struct{}{}
		if err := requireNumber(fmt.Sprintf("moving-average %d", average.Window), average.Field); err != nil {
			return err
		}
	}
	for _, reference := range temporal.ReferenceLines {
		if math.IsNaN(reference.Value) || math.IsInf(reference.Value, 0) {
			return fmt.Errorf("panel %s reference-line value must be finite", panel.ID)
		}
	}
	if period := temporal.Period; period != nil {
		if strings.TrimSpace(period.Category) == "" {
			return fmt.Errorf("panel %s incomplete-period category is required", panel.ID)
		}
		switch period.State {
		case TemporalPeriodYTD, TemporalPeriodAnnualized:
		default:
			return fmt.Errorf("panel %s has unsupported incomplete-period state %q", panel.ID, period.State)
		}
		if period.AnnualizedField != "" {
			if err := requireNumber("annualized estimate", period.AnnualizedField); err != nil {
				return err
			}
		}
	}
	for _, annotation := range temporal.Annotations {
		if strings.TrimSpace(annotation.At) == "" || strings.TrimSpace(annotation.Label) == "" {
			return fmt.Errorf("panel %s time annotation requires at and label", panel.ID)
		}
	}
	if forecast := temporal.Forecast; forecast != nil {
		if strings.TrimSpace(forecast.Start) == "" {
			return fmt.Errorf("panel %s forecast start is required", panel.ID)
		}
		for role, field := range map[string]string{
			"forecast value": forecast.ValueField,
			"forecast lower": forecast.LowerField,
			"forecast upper": forecast.UpperField,
		} {
			if err := requireNumber(role, field); err != nil {
				return err
			}
		}
	}
	return nil
}
