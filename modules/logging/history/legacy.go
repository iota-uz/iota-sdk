package history

import "github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"

// LegacyActionLogType is the event type of records migrated from action_logs.
const LegacyActionLogType = "logging.action_log"

func LegacyActionLogDefinition() Definition {
	return Definition{
		Type:            LegacyActionLogType,
		Version:         1,
		Category:        businessevent.CategoryLegacy,
		TitleKey:        "BusinessHistory.Events.LegacyActionLog.Title",
		DescriptionKey:  "BusinessHistory.Events.LegacyActionLog.Summary",
		SubjectOptional: true,
	}
}
