// Package filtercontext composes immutable analytical scopes without executing queries.
package filtercontext

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const URLParam = "lens-filters"
const MaxBytes = 16384

type ContextPeriod struct {
	Mode   string `json:"mode"`
	Start  string `json:"start,omitempty"`
	End    string `json:"end,omitempty"`
	Days   int    `json:"days,omitempty"`
	Months int    `json:"months,omitempty"`
}

type ContextConstraint struct {
	Dimension string   `json:"dimension"`
	Values    []string `json:"values"`
}

type ContextFilters struct {
	Period      *ContextPeriod      `json:"period,omitempty"`
	Constraints []ContextConstraint `json:"constraints,omitempty"`
}

type ContextCalendar struct {
	Timezone      string        `json:"timezone"`
	AsOf          string        `json:"asOf"`
	DefaultPeriod ContextPeriod `json:"defaultPeriod"`
}

type ContextOrigin struct {
	Scope string `json:"scope"`
	ID    string `json:"id"`
}

type ContextLayer struct {
	Origin  ContextOrigin  `json:"origin"`
	Filters ContextFilters `json:"filters"`
}

type ContextInput struct {
	SupportsPeriod      *bool           `json:"supportsPeriod,omitempty"`
	Calendar            ContextCalendar `json:"calendar"`
	Layers              []ContextLayer  `json:"layers"`
	SupportedDimensions []string        `json:"supportedDimensions"`
}

// ContextBoard is temporary composition state. Saved report defaults stay on the host definitions.
type ContextBoard struct {
	Calendar  ContextCalendar           `json:"calendar"`
	Dashboard ContextFilters            `json:"dashboard"`
	Cards     map[string]ContextFilters `json:"cards,omitempty"`
	Sections  map[string]ContextFilters `json:"sections,omitempty"`
	Groups    map[string][]ContextLayer `json:"groups,omitempty"`
}

type ContextEditor struct {
	URL             string            `json:"url"`
	Value           ContextBoard      `json:"value"`
	Saved           ContextBoard      `json:"saved"`
	DimensionLabels map[string]string `json:"dimensionLabels,omitempty"`
}

type ContextRestriction struct {
	Dimension string        `json:"dimension"`
	Values    []string      `json:"values"`
	Origin    ContextOrigin `json:"origin"`
}

type ContextRange struct {
	Selection ContextPeriod `json:"selection"`
	Origin    ContextOrigin `json:"origin"`
	Start     string        `json:"start,omitempty"`
	End       string        `json:"end,omitempty"`
}

type EffectiveContext struct {
	Identity     string               `json:"identity"`
	Calendar     ContextCalendar      `json:"calendar"`
	Period       ContextRange         `json:"period"`
	Restrictions []ContextRestriction `json:"restrictions"`
	Intersection []ContextConstraint  `json:"intersection"`
	Conflicts    []string             `json:"conflicts"`
	Unsupported  []ContextRestriction `json:"unsupported"`
	Input        ContextInput         `json:"input"`
}

func invalid(message string) error {
	return serrors.New(serrors.Invalid, message).WithOp("lens.filtercontext")
}

func Resolve(input ContextInput) (EffectiveContext, error) {
	data, err := json.Marshal(input)
	if err != nil || len(data) > MaxBytes {
		return EffectiveContext{}, invalid("filter context exceeds its limit")
	}
	var copied ContextInput
	if err = json.Unmarshal(data, &copied); err != nil {
		return EffectiveContext{}, err
	}
	rank := map[string]int{"report": 0, "dashboard": 1, "section": 2, "card": 3, "group": 4}
	slices.SortStableFunc(copied.Layers, func(a, b ContextLayer) int { return rank[a.Origin.Scope] - rank[b.Origin.Scope] })
	if copied.SupportedDimensions == nil {
		copied.SupportedDimensions = []string{}
	}
	sort.Strings(copied.SupportedDimensions)
	copied.SupportedDimensions = slices.Compact(copied.SupportedDimensions)
	location, err := time.LoadLocation(copied.Calendar.Timezone)
	if err != nil || copied.Calendar.Timezone == "" {
		return EffectiveContext{}, invalid("host timezone is required")
	}
	asOf, err := time.Parse(time.RFC3339, copied.Calendar.AsOf)
	if err != nil {
		return EffectiveContext{}, invalid("host as-of is required")
	}
	out := EffectiveContext{Calendar: copied.Calendar, Input: copied, Restrictions: []ContextRestriction{}, Intersection: []ContextConstraint{}, Conflicts: []string{}, Unsupported: []ContextRestriction{}}
	period := copied.Calendar.DefaultPeriod
	origin := ContextOrigin{Scope: "host", ID: "calendar"}
	sets := map[string][]string{}
	for i := range copied.Layers {
		layer := &copied.Layers[i]
		if err := ValidateFilters(layer.Filters, layer.Origin.Scope == "dashboard"); err != nil {
			return EffectiveContext{}, err
		}
		if !slices.Contains([]string{"report", "dashboard", "card", "section", "group"}, layer.Origin.Scope) || layer.Origin.ID == "" {
			return EffectiveContext{}, invalid("invalid filter origin")
		}
		if p := layer.Filters.Period; p != nil && p.Mode != "reports" {
			period, origin = *p, layer.Origin
		}
		if p := layer.Filters.Period; p != nil && p.Mode == "reports" && layer.Origin.Scope != "dashboard" {
			return EffectiveContext{}, invalid("report-period mode belongs to the dashboard")
		}
		for j := range layer.Filters.Constraints {
			constraint := &layer.Filters.Constraints[j]
			if strings.TrimSpace(constraint.Dimension) == "" || len(constraint.Values) == 0 {
				return EffectiveContext{}, invalid("a dimension constraint needs stable values")
			}
			values := slices.Clone(constraint.Values)
			for _, value := range values {
				if value == "" {
					return EffectiveContext{}, invalid("constraint values cannot be empty")
				}
			}
			sort.Strings(values)
			values = slices.Compact(values)
			constraint.Values = values
			restriction := ContextRestriction{constraint.Dimension, values, layer.Origin}
			out.Restrictions = append(out.Restrictions, restriction)
			if !slices.Contains(copied.SupportedDimensions, constraint.Dimension) {
				out.Unsupported = append(out.Unsupported, restriction)
			}
			if previous, exists := sets[constraint.Dimension]; exists {
				intersection := []string{}
				for _, value := range previous {
					if slices.Contains(values, value) {
						intersection = append(intersection, value)
					}
				}
				sets[constraint.Dimension] = intersection
			} else {
				sets[constraint.Dimension] = values
			}
		}
	}
	start, end, err := resolvePeriod(period, asOf.In(location))
	if err != nil {
		return EffectiveContext{}, err
	}
	out.Period = ContextRange{Selection: period, Origin: origin, Start: start, End: end}
	if copied.SupportsPeriod != nil && !*copied.SupportsPeriod && origin.Scope != "host" {
		out.Unsupported = append(out.Unsupported, ContextRestriction{Dimension: "period", Values: []string{period.Mode}, Origin: origin})
	}
	keys := make([]string, 0, len(sets))
	for key := range sets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out.Intersection = append(out.Intersection, ContextConstraint{key, sets[key]})
		if len(sets[key]) == 0 {
			out.Conflicts = append(out.Conflicts, key)
		}
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return EffectiveContext{}, err
	}
	digest := sha256.Sum256(encoded)
	out.Identity = hex.EncodeToString(digest[:])
	return out, nil
}

func resolvePeriod(p ContextPeriod, today time.Time) (string, string, error) {
	switch p.Mode {
	case "all":
		if p.Start != "" || p.End != "" || p.Days != 0 || p.Months != 0 {
			return "", "", invalid("all-time cannot have boundaries")
		}
		return "", "", nil
	case "fixed":
		start, a := time.Parse("2006-01-02", p.Start)
		end, b := time.Parse("2006-01-02", p.End)
		if a != nil || b != nil || end.Before(start) || p.Days != 0 || p.Months != 0 {
			return "", "", invalid("invalid fixed period")
		}
		return p.Start, p.End, nil
	case "relative":
		if p.Months > 0 && p.Months <= 120 && p.Days == 0 && p.Start == "" && p.End == "" {
			return today.AddDate(0, -p.Months, 1).Format("2006-01-02"), today.Format("2006-01-02"), nil
		}
		if p.Days < 1 || p.Days > 3660 || p.Months != 0 || p.Start != "" || p.End != "" {
			return "", "", invalid("invalid relative period")
		}
		return today.AddDate(0, 0, 1-p.Days).Format("2006-01-02"), today.Format("2006-01-02"), nil
	default:
		return "", "", invalid("a concrete default period is required")
	}
}

func (e EffectiveContext) Ready() bool { return len(e.Conflicts) == 0 && len(e.Unsupported) == 0 }

// ValidateFilters validates selections even when a later layer overrides their period.
func ValidateFilters(filters ContextFilters, allowReports bool) error {
	if p := filters.Period; p != nil {
		if p.Mode == "reports" {
			if !allowReports || p.Start != "" || p.End != "" || p.Days != 0 || p.Months != 0 {
				return invalid("invalid inherited report period")
			}
		} else if _, _, err := resolvePeriod(*p, time.Now()); err != nil {
			return err
		}
	}
	for _, constraint := range filters.Constraints {
		if strings.TrimSpace(constraint.Dimension) == "" || len(constraint.Values) == 0 {
			return invalid("a dimension constraint needs stable values")
		}
		for _, value := range constraint.Values {
			if value == "" {
				return invalid("constraint values cannot be empty")
			}
		}
	}
	return nil
}

func ValidateBoard(board ContextBoard) error {
	if _, err := Resolve(ContextInput{Calendar: board.Calendar}); err != nil {
		return err
	}
	if err := ValidateFilters(board.Dashboard, true); err != nil {
		return err
	}
	for _, scopes := range []map[string]ContextFilters{board.Cards, board.Sections} {
		for id, filters := range scopes {
			if id == "" {
				return invalid("scope ID is required")
			}
			if err := ValidateFilters(filters, false); err != nil {
				return err
			}
		}
	}
	for id, layers := range board.Groups {
		if id == "" {
			return invalid("card ID is required")
		}
		for _, layer := range layers {
			if layer.Origin.Scope != "group" || layer.Origin.ID == "" {
				return invalid("invalid group origin")
			}
			if err := ValidateFilters(layer.Filters, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func EncodeBoard(board ContextBoard) (string, error) {
	if err := ValidateBoard(board); err != nil {
		return "", err
	}
	data, err := json.Marshal(board)
	if err != nil {
		return "", err
	}
	if len(data) > MaxBytes {
		return "", invalid("filter context exceeds its limit")
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func DecodeBoard(value string) (ContextBoard, error) {
	var board ContextBoard
	if len(value) > base64.RawURLEncoding.EncodedLen(MaxBytes) {
		return board, invalid("encoded context exceeds its limit")
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return board, invalid("invalid encoded filter context")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&board); err != nil {
		return board, invalid("invalid filter context")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return board, invalid("trailing filter context data")
	}
	return board, ValidateBoard(board)
}

func Encode(input ContextInput) (string, error) {
	resolved, err := Resolve(input)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(resolved.Input)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func Decode(value string) (ContextInput, error) {
	if len(value) > base64.RawURLEncoding.EncodedLen(MaxBytes) {
		return ContextInput{}, invalid("encoded context exceeds its limit")
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return ContextInput{}, invalid("invalid encoded filter context")
	}
	var input ContextInput
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&input); err != nil {
		return ContextInput{}, invalid("invalid filter context")
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return ContextInput{}, invalid("trailing filter context data")
	}
	resolved, err := Resolve(input)
	return resolved.Input, err
}

func PushGroup(input ContextInput, id string, filters ContextFilters) (EffectiveContext, error) {
	input.Layers = append(slices.Clone(input.Layers), ContextLayer{ContextOrigin{"group", id}, filters})
	return Resolve(input)
}

func Back(input ContextInput) (EffectiveContext, error) {
	input.Layers = slices.Clone(input.Layers)
	for i := len(input.Layers) - 1; i >= 0; i-- {
		if input.Layers[i].Origin.Scope == "group" {
			input.Layers = append(input.Layers[:i], input.Layers[i+1:]...)
			break
		}
	}
	return Resolve(input)
}
