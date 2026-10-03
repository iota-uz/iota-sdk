// Package testenv provides opt-in infrastructure for isolated browser tests.
package testenv

import (
	"context"
	"time"
)

type Error struct {
	Code              string   `json:"code"`
	Message           string   `json:"message"`
	Operation         string   `json:"operation,omitempty"`
	EnvironmentID     string   `json:"environmentId,omitempty"`
	ArtifactDirectory string   `json:"artifactDirectory,omitempty"`
	Causes            []*Error `json:"causes,omitempty"`
	cause             error
}

func (e *Error) Error() string           { return e.Code + ": " + e.Message }
func (e *Error) Unwrap() error           { return e.cause }
func failure(code, message string) error { return &Error{Code: code, Message: message} }

type Definition struct {
	Name                 string   `json:"name"`
	Version              string   `json:"version"`
	InputSchema          any      `json:"inputSchema"`
	OutputSchema         any      `json:"outputSchema"`
	RequiredCapabilities []string `json:"requiredCapabilities"`
	Isolation            string   `json:"isolation"`
}
type Input struct {
	Name    string         `json:"name"`
	Version string         `json:"version"`
	ScopeID string         `json:"scopeId"`
	Seed    string         `json:"seed"`
	Now     string         `json:"now"`
	Params  map[string]any `json:"params"`
}
type EntityRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}
type Result struct {
	ScopeID string               `json:"scopeId"`
	Name    string               `json:"name"`
	Version string               `json:"version"`
	Seed    string               `json:"seed"`
	Now     string               `json:"now"`
	Refs    map[string]EntityRef `json:"refs"`
	Data    map[string]any       `json:"data"`
}
type PrepareFunc func(context.Context, Input) (Result, error)
type CleanupFunc func(context.Context, string) error

type Spec struct {
	RunID                string   `json:"runId"`
	Slot                 string   `json:"slot"`
	Isolation            string   `json:"isolation"`
	SchemaFingerprint    string   `json:"schemaFingerprint"`
	BaselineFingerprint  string   `json:"baselineFingerprint"`
	RequiredCapabilities []string `json:"requiredCapabilities"`
}
type Descriptor struct {
	EnvironmentID       string   `json:"environmentId"`
	BaseURL             string   `json:"baseURL"`
	BuildRevision       string   `json:"buildRevision"`
	SchemaFingerprint   string   `json:"schemaFingerprint"`
	BaselineFingerprint string   `json:"baselineFingerprint"`
	Capabilities        []string `json:"capabilities"`
	ArtifactDirectory   string   `json:"artifactDirectory"`
}

// Adapter owns the application's processes, databases and external resources.
// Start must return a handle even on partial failure so Stop can compensate.
type Adapter interface {
	Start(context.Context, Spec, string) (Descriptor, error)
	Ready(context.Context, Descriptor) error
	Stop(context.Context, Descriptor) error
}

type Clock interface{ Now() time.Time }
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }
