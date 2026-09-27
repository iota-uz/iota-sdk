package controllers

import (
	"net/http"

	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
)

func respondPrivilegeDenied(w http.ResponseWriter, r *http.Request, err error) bool {
	if !services.IsPrivilegeDenied(err) {
		return false
	}

	title := "Action not allowed"
	message := "This action is not allowed. Changes were not saved."
	if pageCtx, ok := composables.TryUsePageCtx(r.Context()); ok {
		title = pageCtx.TSafe("Authorization.Errors.Title")
		message = pageCtx.TSafe(services.PrivilegeDenialLocaleKey(err))
		if title == "" {
			title = "Action not allowed"
		}
		if message == "" {
			message = "This action is not allowed. Changes were not saved."
		}
	}

	if htmx.IsHxRequest(r) {
		htmx.ToastError(w, title, message)
	}
	http.Error(w, message, http.StatusForbidden)
	return true
}
