package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/composables"
)

func buildDownloadURL(ctx context.Context, baseURL, filename string) string {
	cleanFilename := strings.TrimLeft(strings.TrimSpace(filename), "/")
	if cleanFilename == "" {
		return ""
	}

	trimmedBase := strings.TrimSpace(baseURL)
	if trimmedBase == "" {
		return resolveDownloadURL(ctx, "/"+cleanFilename)
	}

	if isAbsoluteURL(trimmedBase) {
		return strings.TrimRight(trimmedBase, "/") + "/" + cleanFilename
	}

	relativePath := "/" + strings.Trim(trimmedBase, "/") + "/" + cleanFilename
	return resolveDownloadURL(ctx, relativePath)
}

func resolveDownloadURL(ctx context.Context, rawURL string) string {
	cleanURL := strings.TrimSpace(rawURL)
	if cleanURL == "" {
		return ""
	}

	if isAbsoluteURL(cleanURL) {
		return cleanURL
	}

	if !strings.HasPrefix(cleanURL, "/") {
		cleanURL = "/" + cleanURL
	}

	req, ok := composables.UseRequest(ctx)
	if !ok || req == nil {
		return cleanURL
	}

	return absoluteURLFromRequest(req, cleanURL)
}

func absoluteURLFromRequest(req *http.Request, resourcePath string) string {
	host := requestHost(req)
	if host == "" {
		return resourcePath
	}
	return fmt.Sprintf("%s://%s%s", requestScheme(req), host, resourcePath)
}

func requestHost(req *http.Request) string {
	forwardedHost := strings.TrimSpace(req.Header.Get("X-Forwarded-Host"))
	if forwardedHost != "" {
		parts := strings.Split(forwardedHost, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	if req.Host != "" {
		return req.Host
	}

	if req.URL != nil {
		return req.URL.Host
	}

	return ""
}

func requestScheme(req *http.Request) string {
	forwardedProto := strings.TrimSpace(req.Header.Get("X-Forwarded-Proto"))
	if forwardedProto != "" {
		parts := strings.Split(forwardedProto, ",")
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			return strings.TrimSpace(parts[0])
		}
	}

	if req.URL != nil && req.URL.Scheme != "" {
		return req.URL.Scheme
	}

	if req.TLS != nil {
		return "https"
	}

	return "http"
}

func isAbsoluteURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	return parsed.IsAbs() && parsed.Host != ""
}
