package application

// RuntimeProfile describes the lifecycle expectations of the current process.
// Server-style profiles may start long-lived workers; bootstrap-style profiles
// must stay finite so one-shot commands can exit cleanly.
type RuntimeProfile string

const (
	RuntimeProfileServer    RuntimeProfile = "server"
	RuntimeProfileBootstrap RuntimeProfile = "bootstrap"
)

func normalizeRuntimeProfile(profile RuntimeProfile) RuntimeProfile {
	if profile == "" {
		return RuntimeProfileServer
	}
	return profile
}

func (p RuntimeProfile) AllowsBackgroundWork() bool {
	return normalizeRuntimeProfile(p) == RuntimeProfileServer
}
