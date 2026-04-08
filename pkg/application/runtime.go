package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// RuntimeTag selects which long-running runtime components should start.
type RuntimeTag string

const (
	RuntimeTagAPI    RuntimeTag = "api"
	RuntimeTagWorker RuntimeTag = "worker"
)

func normalizeRuntimeTags(tags []RuntimeTag) ([]RuntimeTag, error) {
	const op serrors.Op = "application.normalizeRuntimeTags"

	if len(tags) == 0 {
		return nil, nil
	}

	normalized := make([]RuntimeTag, 0, len(tags))
	seen := make(map[RuntimeTag]struct{}, len(tags))
	for _, tag := range tags {
		trimmed := RuntimeTag(strings.TrimSpace(string(tag)))
		if trimmed == "" {
			continue
		}
		switch trimmed {
		case RuntimeTagAPI, RuntimeTagWorker:
		default:
			return nil, serrors.E(op, serrors.Invalid, fmt.Sprintf("invalid runtime tag %q", tag))
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	slices.Sort(normalized)
	return normalized, nil
}

func runtimeTagSet(tags []RuntimeTag) map[RuntimeTag]struct{} {
	set := make(map[RuntimeTag]struct{}, len(tags))
	for _, tag := range tags {
		set[tag] = struct{}{}
	}
	return set
}

func runtimeTagsEqual(left, right []RuntimeTag) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func formatRuntimeTags(tags []RuntimeTag) string {
	if len(tags) == 0 {
		return "[]"
	}
	values := make([]string, 0, len(tags))
	for _, tag := range tags {
		values = append(values, string(tag))
	}
	return "[" + strings.Join(values, ", ") + "]"
}

type RuntimeComponent interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// RuntimeRegistration binds a runtime component to one or more runtime tags.
type RuntimeRegistration struct {
	Component RuntimeComponent
	Tags      []RuntimeTag
}

func (r RuntimeRegistration) AppliesTo(activeTags map[RuntimeTag]struct{}) bool {
	if r.Component == nil {
		return false
	}
	if len(r.Tags) == 0 {
		return true
	}
	for _, candidate := range r.Tags {
		if _, ok := activeTags[candidate]; ok {
			return true
		}
	}
	return false
}
