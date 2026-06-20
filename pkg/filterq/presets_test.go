package filterq_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/filterq"
)

func date(loc *time.Location, y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func TestPresetRange(t *testing.T) {
	t.Parallel()
	tashkent, err := time.LoadLocation("Asia/Tashkent")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		preset   filterq.DatePreset
		now      time.Time
		from, to time.Time
	}{
		{
			name:   "this month mid-month",
			preset: filterq.PresetThisMonth,
			now:    time.Date(2026, 6, 12, 15, 30, 0, 0, time.UTC),
			from:   date(time.UTC, 2026, 6, 1),
			to:     date(time.UTC, 2026, 6, 30),
		},
		{
			name:   "this month February leap year",
			preset: filterq.PresetThisMonth,
			now:    time.Date(2024, 2, 29, 23, 59, 0, 0, time.UTC),
			from:   date(time.UTC, 2024, 2, 1),
			to:     date(time.UTC, 2024, 2, 29),
		},
		{
			name:   "last month across year boundary",
			preset: filterq.PresetLastMonth,
			now:    time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			from:   date(time.UTC, 2025, 12, 1),
			to:     date(time.UTC, 2025, 12, 31),
		},
		{
			name:   "last month after 31st (March 31 -> February)",
			preset: filterq.PresetLastMonth,
			now:    time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC),
			from:   date(time.UTC, 2026, 2, 1),
			to:     date(time.UTC, 2026, 2, 28),
		},
		{
			name:   "last 30 days inclusive of today",
			preset: filterq.PresetLast30D,
			now:    time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC),
			from:   date(time.UTC, 2026, 5, 14),
			to:     date(time.UTC, 2026, 6, 12),
		},
		{
			name:   "next 30 days inclusive of today",
			preset: filterq.PresetNext30D,
			now:    time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC),
			from:   date(time.UTC, 2026, 6, 12),
			to:     date(time.UTC, 2026, 7, 11),
		},
		{
			name:   "this year",
			preset: filterq.PresetThisYear,
			now:    time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC),
			from:   date(time.UTC, 2026, 1, 1),
			to:     date(time.UTC, 2026, 12, 31),
		},
		{
			name:   "last year",
			preset: filterq.PresetLastYear,
			now:    time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC),
			from:   date(time.UTC, 2025, 1, 1),
			to:     date(time.UTC, 2025, 12, 31),
		},
		{
			name:   "non-UTC location preserved",
			preset: filterq.PresetThisMonth,
			now:    time.Date(2026, 6, 12, 1, 0, 0, 0, tashkent),
			from:   date(tashkent, 2026, 6, 1),
			to:     date(tashkent, 2026, 6, 30),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			from, to := tc.preset.Range(tc.now)
			if !from.Equal(tc.from) || from.Location() != tc.from.Location() {
				t.Errorf("from = %v, want %v", from, tc.from)
			}
			if !to.Equal(tc.to) || to.Location() != tc.to.Location() {
				t.Errorf("to = %v, want %v", to, tc.to)
			}
		})
	}
}

func TestParsePresetValue(t *testing.T) {
	t.Parallel()
	if p, ok := filterq.ParsePresetValue("preset:this_year"); !ok || p != filterq.PresetThisYear {
		t.Errorf("ParsePresetValue = %v, %v", p, ok)
	}
	if _, ok := filterq.ParsePresetValue("preset:bogus"); ok {
		t.Error("bogus preset must not parse")
	}
	if _, ok := filterq.ParsePresetValue("2026-06-01"); ok {
		t.Error("plain date must not parse as preset")
	}
}

func TestConditionPresetRequiresBetween(t *testing.T) {
	t.Parallel()
	// A preset value only counts as a preset under OpBetween; with any other
	// operator it must not resolve (it would otherwise mis-render in chips).
	if _, ok := (filterq.Condition{Op: filterq.OpBetween, Values: []string{"preset:next_30d"}}).Preset(); !ok {
		t.Error("OpBetween preset must resolve")
	}
	for _, op := range []filterq.Operator{filterq.OpOn, filterq.OpBefore, filterq.OpAfter, filterq.OpIs} {
		if _, ok := (filterq.Condition{Op: op, Values: []string{"preset:next_30d"}}).Preset(); ok {
			t.Errorf("preset must not resolve for operator %q", op)
		}
	}
}

func TestConditionDateRange(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)

	c := filterq.Condition{Field: "d", Op: filterq.OpBetween, Values: []string{"preset:this_year"}}
	from, to, ok := c.DateRange(now)
	if !ok || !from.Equal(date(time.UTC, 2026, 1, 1)) || !to.Equal(date(time.UTC, 2026, 12, 31)) {
		t.Errorf("preset DateRange = %v..%v ok=%v", from, to, ok)
	}

	c = filterq.Condition{Field: "d", Op: filterq.OpBetween, Values: []string{"2026-06-01", "2026-06-30"}}
	from, to, ok = c.DateRange(now)
	if !ok || !from.Equal(date(time.UTC, 2026, 6, 1)) || !to.Equal(date(time.UTC, 2026, 6, 30)) {
		t.Errorf("explicit DateRange = %v..%v ok=%v", from, to, ok)
	}

	for _, bad := range []filterq.Condition{
		{Field: "d", Op: filterq.OpOn, Values: []string{"2026-06-01"}},
		{Field: "d", Op: filterq.OpBetween, Values: []string{"2026-06-01"}},
		{Field: "d", Op: filterq.OpBetween, Values: []string{"junk", "2026-06-30"}},
	} {
		if _, _, ok := bad.DateRange(now); ok {
			t.Errorf("DateRange(%#v) must not resolve", bad)
		}
	}
}

func TestDefaultOperators(t *testing.T) {
	t.Parallel()
	cases := map[filterq.FieldType][]filterq.Operator{
		filterq.FieldTypeReference: {filterq.OpIs, filterq.OpIsNot},
		filterq.FieldTypeDate:      {filterq.OpOn, filterq.OpBefore, filterq.OpAfter, filterq.OpBetween},
		filterq.FieldTypeNumber:    {filterq.OpEq, filterq.OpGt, filterq.OpLt, filterq.OpBetween},
		filterq.FieldTypeBool:      {filterq.OpIs},
	}
	for ft, want := range cases {
		got := filterq.DefaultOperators(ft)
		if len(got) != len(want) {
			t.Errorf("DefaultOperators(%s) = %v, want %v", ft, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("DefaultOperators(%s)[%d] = %v, want %v", ft, i, got[i], want[i])
			}
		}
	}
}
