// Package notify adapts the extension runtime to the core notification
// service: explicit notices are enqueued atomically in the caller's tx,
// package notices appear in the notification catalog, templates render
// through the Node pool after commit, and package contact resolvers serve
// record recipients.
package notify

import (
	"regexp"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var versionSegment = regexp.MustCompile(`^v[1-9][0-9]*$`)

func snake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 && s[i-1] != '_' && (s[i-1] < 'A' || s[i-1] > 'Z') {
				b.WriteByte('_')
			}
			b.WriteRune(r + ('a' - 'A'))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// EventKey maps a notice resource ID ("acme.insurance.renewalReminder.v1") to
// the notification catalog key shape (lowercase segments, trailing .vN).
func EventKey(noticeID string) string {
	segments := strings.Split(noticeID, ".")
	for i := range segments {
		segments[i] = snake(segments[i])
	}
	if !versionSegment.MatchString(segments[len(segments)-1]) {
		segments = append(segments, "v1")
	}
	return strings.Join(segments, ".")
}

// Runtime channel names (template keys) and core channels.
func coreChannel(name string) (notifications.Channel, error) {
	switch name {
	case "sms":
		return notifications.ChannelSMS, nil
	case "email":
		return notifications.ChannelEmail, nil
	case "inApp", "in_app":
		return notifications.ChannelInApp, nil
	case "push":
		return notifications.ChannelPush, nil
	}
	return "", serrors.New(serrors.Invalid, "unknown notification channel "+name)
}

func templateChannel(c notifications.Channel) string {
	if c == notifications.ChannelInApp {
		return "inApp"
	}
	return string(c)
}

// SnapshotRef is the immutable reference queued notices keep: package@version.
func SnapshotRef(pkg, version string) string { return pkg + "@" + version }

// SplitRef parses a SnapshotRef into its package and version; ok is false when
// ref is not of the form package@version.
func SplitRef(ref string) (string, string, bool) {
	i := strings.LastIndex(ref, "@")
	if i <= 0 || i == len(ref)-1 {
		return "", "", false
	}
	return ref[:i], ref[i+1:], true
}

// SourceName is the notification catalog source of one package.
func SourceName(pkg string) string { return "extruntime:" + pkg }
