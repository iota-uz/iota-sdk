// Package middleware provides this package.
package middleware

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/headers"
	"github.com/sirupsen/logrus"
)

// LogTransport is a http.RoundTripper middleware for logging outgoing HTTP requests/responses.
type LogTransport struct {
	Base            http.RoundTripper
	Conf            *headers.Config
	Logger          *logrus.Logger
	LogRequestBody  bool
	LogResponseBody bool
	Name            string
}

// NewLogTransport constructs a new LogTransport with given options.
func NewLogTransport(logger *logrus.Logger, conf *headers.Config, logRequestBody, logResponseBody bool, name string) *LogTransport {
	if name == "" {
		name = "client"
	}
	return &LogTransport{
		Base:            http.DefaultTransport,
		Conf:            conf,
		Logger:          logger,
		LogRequestBody:  logRequestBody,
		LogResponseBody: logResponseBody,
		Name:            name,
	}
}

// RoundTrip implements http.RoundTripper.
func (l *LogTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	requestIDHeader := "X-Request-ID"
	if l.Conf != nil && l.Conf.RequestID != "" {
		requestIDHeader = l.Conf.RequestID
	}

	// Extract or generate request-id
	var requestID string
	if req.Header.Get(requestIDHeader) != "" {
		requestID = req.Header.Get(requestIDHeader)
	} else {
		requestID = uuid.New().String()
		req.Header.Set(requestIDHeader, requestID)
	}

	// Log request body
	var reqBody string
	if req.Body != nil && l.LogRequestBody && shouldLogBody(req.Header.Get("Content-Type")) {
		bodyBytes, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		reqBody = string(bodyBytes)
	}

	l.Logger.WithFields(logrus.Fields{
		"type":           "http-client-request",
		"request-id":     requestID,
		"method":         req.Method,
		"url":            req.URL.String(),
		"origin":         req.URL.Scheme + "://" + req.URL.Host,
		"headers":        req.Header,
		"request-body":   parseBody(reqBody, req.Header.Get("Content-Type")),
		"request-length": len(reqBody),
		"client":         l.Name,
	}).Info("HTTP client request started")

	// Perform request
	resp, err := l.Base.RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		l.Logger.WithFields(logrus.Fields{
			"type":       "http-client-error",
			"request-id": requestID,
			"method":     req.Method,
			"url":        req.URL.String(),
			"origin":     req.URL.Scheme + "://" + req.URL.Host,
			"error":      err,
			"duration":   duration,
			"client":     l.Name,
		}).Error("HTTP client request failed")
		return nil, err
	}

	// Log response body
	var respBody string
	if resp.Body != nil && l.LogResponseBody && shouldLogBody(resp.Header.Get("Content-Type")) {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		respBody = string(bodyBytes)
	}

	l.Logger.WithFields(logrus.Fields{
		"type":            "http-client-response",
		"request-id":      requestID,
		"method":          req.Method,
		"url":             req.URL.String(),
		"origin":          req.URL.Scheme + "://" + req.URL.Host,
		"status":          resp.Status,
		"status_code":     resp.StatusCode,
		"headers":         resp.Header,
		"response-body":   parseBody(respBody, resp.Header.Get("Content-Type")),
		"response-length": len(respBody),
		"duration":        duration,
		"client":          l.Name,
	}).Info("HTTP client response received")

	return resp, nil
}

// parseBody tries to parse JSON/XML or returns raw string.
func parseBody(raw string, contentType string) interface{} {
	switch {
	case strings.Contains(contentType, "application/json"):
		var out interface{}
		if err := json.Unmarshal([]byte(raw), &out); err == nil {
			return out
		}
	case strings.Contains(contentType, "application/xml"), strings.Contains(contentType, "text/xml"):
		var out interface{}
		if err := xml.Unmarshal([]byte(raw), &out); err == nil {
			return out
		}
	}
	return raw
}
