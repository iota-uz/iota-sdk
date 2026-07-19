package serve

import (
	"net/url"

	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

const (
	paramLocale    = "__lens_serve_locale"
	paramTimezone  = "__lens_serve_timezone"
	paramPath      = "__lens_serve_path"
	paramRequest   = "__lens_serve_request"
	paramDataScope = "__lens_serve_data_scope"
	paramNamespace = "__lens_serve_namespace"
)

func freezeParams(result *lensruntime.DashboardResult, request lensruntime.Request) map[string]any {
	params := cloneParams(result.Variables)
	locale := result.Locale
	if locale == "" {
		locale = request.Locale
	}
	timezone := result.Timezone
	if timezone == "" {
		timezone = request.Timezone
	}
	requestPath := result.RequestPath
	if requestPath == "" {
		requestPath = request.Path
	}
	values := result.Request
	if values == nil {
		values = request.Request
	}
	params[paramLocale] = locale
	params[paramTimezone] = timezone
	params[paramPath] = requestPath
	params[paramRequest] = freezeValues(values)
	params[paramDataScope] = request.DataScope
	params[paramNamespace] = request.Namespace
	return params
}

func thawRuntimeRequest(request lensruntime.Request, params map[string]any) lensruntime.Request {
	request.Locale = stringParam(params, paramLocale, request.Locale)
	request.Timezone = stringParam(params, paramTimezone, request.Timezone)
	request.Path = stringParam(params, paramPath, request.Path)
	request.DataScope = stringParam(params, paramDataScope, request.DataScope)
	request.Namespace = stringParam(params, paramNamespace, request.Namespace)
	if frozen, ok := params[paramRequest].(map[string]any); ok {
		request.Request = thawValues(frozen)
	}
	return request
}

func variableParams(params map[string]any) map[string]any {
	result := make(map[string]any, len(params))
	for key, value := range params {
		switch key {
		case paramLocale, paramTimezone, paramPath, paramRequest, paramDataScope, paramNamespace:
			continue
		default:
			result[key] = value
		}
	}
	return result
}

func freezeValues(values url.Values) map[string]any {
	result := make(map[string]any, len(values))
	for key, items := range values {
		frozen := make([]any, len(items))
		for index, item := range items {
			frozen[index] = item
		}
		result[key] = frozen
	}
	return result
}

func thawValues(values map[string]any) url.Values {
	result := make(url.Values, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case string:
			result[key] = []string{typed}
		case []any:
			items := make([]string, 0, len(typed))
			for _, item := range typed {
				if text, ok := item.(string); ok {
					items = append(items, text)
				}
			}
			result[key] = items
		}
	}
	return result
}

func stringParam(params map[string]any, key, fallback string) string {
	if value, ok := params[key].(string); ok {
		return value
	}
	return fallback
}
