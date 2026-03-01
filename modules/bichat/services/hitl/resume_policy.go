// Package hitl provides this package.
package hitl

import "strings"

// ResolveCheckpoint chooses the canonical checkpoint and reports whether user-supplied checkpoint mismatched.
func ResolveCheckpoint(requestedCheckpointID, canonicalCheckpointID string) (string, bool) {
	canonical := strings.TrimSpace(canonicalCheckpointID)
	requested := strings.TrimSpace(requestedCheckpointID)
	if requested == "" {
		return canonical, false
	}
	return canonical, requested != canonical
}
