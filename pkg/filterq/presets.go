package filterq

import (
	"strings"
	"time"
)

// DatePreset is a symbolic, clock-relative date range. Presets serialize as
// `preset:<token>` inside an OpBetween condition so shared links keep their
// intent ("expiring in the next 30 days" stays correct next week) instead of
// freezing concrete dates.
type DatePreset string

const (
	PresetThisMonth DatePreset = "this_month"
	PresetLastMonth DatePreset = "last_month"
	PresetLast30D   DatePreset = "last_30d"
	PresetNext30D   DatePreset = "next_30d"
	PresetThisYear  DatePreset = "this_year"
	PresetLastYear  DatePreset = "last_year"
)

// presetPrefix marks a symbolic preset value inside a condition.
const presetPrefix = "preset:"

// AllPresets lists every preset in canonical display order.
func AllPresets() []DatePreset {
	return []DatePreset{
		PresetThisMonth,
		PresetLastMonth,
		PresetLast30D,
		PresetNext30D,
		PresetThisYear,
		PresetLastYear,
	}
}

// Valid reports whether p is a known preset.
func (p DatePreset) Valid() bool {
	switch p {
	case PresetThisMonth, PresetLastMonth, PresetLast30D, PresetNext30D, PresetThisYear, PresetLastYear:
		return true
	}
	return false
}

// Value returns the codec value form of the preset (`preset:<token>`).
func (p DatePreset) Value() string { return presetPrefix + string(p) }

// Range resolves the preset to inclusive date-only bounds in now's location.
// Both bounds are at midnight; consumers must extend the upper bound to
// end-of-day when comparing against timestamps.
func (p DatePreset) Range(now time.Time) (time.Time, time.Time) {
	y, m, d := now.Date()
	loc := now.Location()
	today := time.Date(y, m, d, 0, 0, 0, 0, loc)
	switch p {
	case PresetThisMonth:
		from := time.Date(y, m, 1, 0, 0, 0, 0, loc)
		return from, from.AddDate(0, 1, -1)
	case PresetLastMonth:
		first := time.Date(y, m, 1, 0, 0, 0, 0, loc)
		return first.AddDate(0, -1, 0), first.AddDate(0, 0, -1)
	case PresetLast30D:
		return today.AddDate(0, 0, -29), today
	case PresetNext30D:
		// Inclusive of today, symmetric with PresetLast30D: a 30-day span
		// (today + 29) rather than 31 days (today + 30).
		return today, today.AddDate(0, 0, 29)
	case PresetThisYear:
		return time.Date(y, 1, 1, 0, 0, 0, 0, loc), time.Date(y, 12, 31, 0, 0, 0, 0, loc)
	case PresetLastYear:
		return time.Date(y-1, 1, 1, 0, 0, 0, 0, loc), time.Date(y-1, 12, 31, 0, 0, 0, 0, loc)
	}
	return time.Time{}, time.Time{}
}

// DateLayout is the codec wire format for explicit date values.
const DateLayout = "2006-01-02"

// ParsePresetValue extracts the preset from a codec value, if it is one.
func ParsePresetValue(v string) (DatePreset, bool) {
	if !strings.HasPrefix(v, presetPrefix) {
		return "", false
	}
	p := DatePreset(strings.TrimPrefix(v, presetPrefix))
	return p, p.Valid()
}

// Preset returns the condition's preset when the condition is a single
// symbolic preset value.
func (c Condition) Preset() (DatePreset, bool) {
	if len(c.Values) != 1 {
		return "", false
	}
	return ParsePresetValue(c.Values[0])
}

// DateRange resolves an OpBetween date condition to inclusive date-only
// bounds: either by resolving its preset against now, or by parsing two
// explicit YYYY-MM-DD values. The bool is false for any other shape.
func (c Condition) DateRange(now time.Time) (time.Time, time.Time, bool) {
	if c.Op != OpBetween {
		return time.Time{}, time.Time{}, false
	}
	if p, isPreset := c.Preset(); isPreset {
		from, to := p.Range(now)
		return from, to, true
	}
	if len(c.Values) != 2 {
		return time.Time{}, time.Time{}, false
	}
	from, err1 := time.ParseInLocation(DateLayout, c.Values[0], now.Location())
	to, err2 := time.ParseInLocation(DateLayout, c.Values[1], now.Location())
	if err1 != nil || err2 != nil {
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}
