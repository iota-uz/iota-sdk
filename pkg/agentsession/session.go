// Package agentsession identifies local-development sessions.
package agentsession

import "github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"

const Audience session.SessionAudience = "agent-dev"

func Allowed(environment string) bool { return enabled && environment == "development" }

func Is(sess session.Session, environment string) bool {
	return sess != nil && sess.Audience() == Audience && Allowed(environment)
}
