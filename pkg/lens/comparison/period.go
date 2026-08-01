package comparison

import (
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
)

// ResolvePeriod normalizes the requested baseline interval for cube-backed and
// pre-materialized dashboards alike.
func ResolvePeriod(
	mode lens.CompareMode,
	anchor lens.DateRangeValue,
	customStart string,
	customEnd string,
	location *time.Location,
) lens.CompareValue {
	if location == nil {
		location = time.UTC
	}
	switch mode {
	case lens.CompareCustom:
		start, startErr := time.ParseInLocation("2006-01-02", strings.TrimSpace(customStart), location)
		end, endErr := time.ParseInLocation("2006-01-02", strings.TrimSpace(customEnd), location)
		if startErr != nil || endErr != nil || end.Before(start) {
			return lens.CompareValue{Mode: lens.CompareOff}
		}
		end = end.Add(24*time.Hour - time.Nanosecond)
		return lens.CompareValue{Mode: mode, Range: lens.DateRangeValue{Mode: "bounded", Start: &start, End: &end}}
	case lens.ComparePreviousPeriod, lens.CompareYearAgo:
		if anchor.Start == nil || anchor.End == nil {
			return lens.CompareValue{Mode: lens.CompareOff}
		}
	default:
		return lens.CompareValue{Mode: lens.CompareOff}
	}

	start, end := *anchor.Start, *anchor.End
	if mode == lens.CompareYearAgo {
		start = addYearsClamped(start, -1)
		end = addYearsClamped(end, -1)
	} else {
		start, end = previousPeriod(start, end)
	}
	return lens.CompareValue{Mode: mode, Range: lens.DateRangeValue{Mode: "bounded", Start: &start, End: &end}}
}

func previousPeriod(start, end time.Time) (time.Time, time.Time) {
	if months, ok := wholeCalendarMonthSpan(start, end); ok {
		shiftedStart := start.AddDate(0, -months, 0)
		return shiftedStart, start.Add(-time.Nanosecond)
	}
	duration := end.Sub(start)
	previousEnd := start.Add(-time.Nanosecond)
	return previousEnd.Add(-duration), previousEnd
}

func wholeCalendarMonthSpan(start, end time.Time) (int, bool) {
	if start.Location() != end.Location() || !start.Equal(startOfDay(start)) || start.Day() != 1 {
		return 0, false
	}
	expectedEnd := time.Date(end.Year(), end.Month()+1, 1, 0, 0, 0, 0, end.Location()).Add(-time.Nanosecond)
	if !end.Equal(expectedEnd) {
		return 0, false
	}
	months := (end.Year()-start.Year())*12 + int(end.Month()-start.Month()) + 1
	return months, months > 0
}

func startOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func addYearsClamped(value time.Time, years int) time.Time {
	year := value.Year() + years
	day := value.Day()
	lastDay := time.Date(year, value.Month()+1, 0, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), value.Location()).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, value.Month(), day, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), value.Location())
}
