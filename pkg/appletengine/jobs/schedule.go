package jobs

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var cronParser = cron.NewParser(
	cron.Minute |
		cron.Hour |
		cron.Dom |
		cron.Month |
		cron.Dow |
		cron.Descriptor,
)

func NextRun(cronExpr string, from time.Time) (time.Time, error) {
	normalized := strings.TrimSpace(cronExpr)
	if normalized == "" {
		return time.Time{}, fmt.Errorf("cron expression is required")
	}
	schedule, err := cronParser.Parse(normalized)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse cron expression: %w", err)
	}
	next := schedule.Next(from.UTC())
	if next.IsZero() {
		return time.Time{}, fmt.Errorf("cron expression has no future runs")
	}
	return next.UTC(), nil
}
