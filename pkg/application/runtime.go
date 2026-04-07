package application

import (
	"context"
	"fmt"
	"strings"
)

// CompositionProfile selects which composition phases a process should execute.
type CompositionProfile string

const (
	CompositionProfileServer      CompositionProfile = "server"
	CompositionProfileAPIOnly     CompositionProfile = "api-only"
	CompositionProfileWorkerOnly  CompositionProfile = "worker-only"
	CompositionProfileBootstrap   CompositionProfile = "bootstrap"
	CompositionProfileMaintenance CompositionProfile = "maintenance"
)

func normalizeCompositionProfile(profile CompositionProfile) (CompositionProfile, error) {
	switch strings.TrimSpace(string(profile)) {
	case "", string(CompositionProfileServer):
		return CompositionProfileServer, nil
	case string(CompositionProfileAPIOnly):
		return CompositionProfileAPIOnly, nil
	case string(CompositionProfileWorkerOnly):
		return CompositionProfileWorkerOnly, nil
	case string(CompositionProfileBootstrap):
		return CompositionProfileBootstrap, nil
	case string(CompositionProfileMaintenance):
		return CompositionProfileMaintenance, nil
	default:
		return "", fmt.Errorf("invalid composition profile %q", profile)
	}
}

func (p CompositionProfile) IncludesTransports() bool {
	switch p {
	case CompositionProfileServer, CompositionProfileAPIOnly:
		return true
	default:
		return false
	}
}

func (p CompositionProfile) StartsRuntime() bool {
	switch p {
	case CompositionProfileServer, CompositionProfileAPIOnly, CompositionProfileWorkerOnly:
		return true
	default:
		return false
	}
}

type RuntimeComponent interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// RuntimeRegistration binds a runtime component to one or more composition profiles.
type RuntimeRegistration struct {
	Component RuntimeComponent
	Profiles  []CompositionProfile
}

func (r RuntimeRegistration) AppliesTo(profile CompositionProfile) bool {
	if r.Component == nil {
		return false
	}
	if len(r.Profiles) == 0 {
		return true
	}
	for _, candidate := range r.Profiles {
		if candidate == profile {
			return true
		}
	}
	return false
}
