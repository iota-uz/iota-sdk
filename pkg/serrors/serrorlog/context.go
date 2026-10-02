package serrorlog

import (
	"context"
	"log/slog"

	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/sirupsen/logrus"
)

// Log emits bounded error attributes through the request logger when present.
func Log(ctx context.Context, err error, event string) {
	entry, ok := ctx.Value(constants.LoggerKey).(*logrus.Entry)
	if !ok || entry == nil || entry.Logger == nil {
		return
	}
	requestID, _ := entry.Data["request-id"].(string)
	if requestID == "" {
		requestID, _ = entry.Data["request_id"].(string)
	}
	fields := logrus.Fields{}
	for _, attr := range Attributes(err, requestID) {
		fields[attr.Key] = attr.Value.Any()
	}
	var level logrus.Level
	switch Level(err) {
	case slog.LevelDebug:
		level = logrus.DebugLevel
	case slog.LevelInfo:
		level = logrus.InfoLevel
	case slog.LevelWarn:
		level = logrus.WarnLevel
	case slog.LevelError:
		level = logrus.ErrorLevel
	default:
		level = logrus.ErrorLevel
	}
	entry.Logger.WithFields(fields).Log(level, event)
}
