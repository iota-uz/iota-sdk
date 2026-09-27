package services

import streamingsvc "github.com/iota-uz/iota-sdk/modules/bichat/services/streaming"

// StreamObservability exposes read-only counters for active BiChat streaming state.
type StreamObservability struct {
	runRegistry *streamingsvc.RunRegistry
}

func NewStreamObservability(runRegistry *streamingsvc.RunRegistry) *StreamObservability {
	return &StreamObservability{runRegistry: runRegistry}
}

func (s *StreamObservability) ActiveRuns() int {
	if s == nil || s.runRegistry == nil {
		return 0
	}

	return s.runRegistry.ActiveRuns()
}

func (s *StreamObservability) ActiveSubscribers() int {
	if s == nil || s.runRegistry == nil {
		return 0
	}

	return s.runRegistry.ActiveSubscribers()
}
