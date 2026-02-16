package spotlight

import "strings"

func ReadinessErrorDetails(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.TrimSpace(msg) == "" {
		return "spotlight readiness failed"
	}
	return msg
}
