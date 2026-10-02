package serrorrpc

import (
	"github.com/iota-uz/go-i18n/v2/i18n"
	serrors "github.com/iota-uz/iota-sdk/pkg/serrors"
)

type Response struct {
	Code    string
	Message string
	Details map[string]any
}

// Project belongs in the dispatcher's general-classification branch, after
// explicit carriers, protocol, middleware and applet sentinel handling.
func Project(err error, l *i18n.Localizer) Response {
	p := serrors.Public(err, l)
	code := p.Code
	if len(p.Fields) > 0 {
		code = "validation"
	} else if serrors.HasCode(err, serrors.PermissionDenied) {
		code = "forbidden"
	}
	var details map[string]any
	if len(p.Fields) > 0 || p.Reason != "" {
		details = map[string]any{}
		if len(p.Fields) > 0 {
			details["fields"] = p.Fields
		}
		if p.Reason != "" {
			details["reason"] = p.Reason
		}
	}
	return Response{Code: code, Message: p.Message, Details: details}
}
