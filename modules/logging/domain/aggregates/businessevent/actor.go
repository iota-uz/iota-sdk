package businessevent

import (
	"strconv"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const ActorSnapshotVersion = 1

type ActorKind string

const (
	ActorUser    ActorKind = "user"
	ActorService ActorKind = "service"
	ActorSystem  ActorKind = "system"
)

// ActorSnapshot is who acted, frozen at the time of the event. It never follows later
// renames or deletion of the account.
type ActorSnapshot struct {
	SchemaVersion int       `json:"schema_version"`
	Kind          ActorKind `json:"kind"`
	UserID        string    `json:"user_id,omitempty"`
	ID            string    `json:"id,omitempty"`
	FirstName     string    `json:"first_name,omitempty"`
	LastName      string    `json:"last_name,omitempty"`
	MiddleName    string    `json:"middle_name,omitempty"`
	DisplayName   string    `json:"display_name,omitempty"`
}

// UserActor copies an allowlisted set of user fields; credentials and other personal data
// are never part of a snapshot, including fields added to users later.
func UserActor(u user.User) ActorSnapshot {
	return ActorSnapshot{
		SchemaVersion: ActorSnapshotVersion,
		Kind:          ActorUser,
		UserID:        strconv.FormatUint(uint64(u.ID()), 10),
		ID:            "",
		FirstName:     u.FirstName(),
		LastName:      u.LastName(),
		MiddleName:    u.MiddleName(),
		DisplayName:   "",
	}
}

func ServiceActor(id, displayName string) ActorSnapshot {
	return executorActor(ActorService, id, displayName)
}

func SystemActor(id, displayName string) ActorSnapshot {
	return executorActor(ActorSystem, id, displayName)
}

func executorActor(kind ActorKind, id, displayName string) ActorSnapshot {
	return ActorSnapshot{
		SchemaVersion: ActorSnapshotVersion,
		Kind:          kind,
		UserID:        "",
		ID:            id,
		FirstName:     "",
		LastName:      "",
		MiddleName:    "",
		DisplayName:   displayName,
	}
}

// Identity is the stable identifier the actor is searched by.
func (a ActorSnapshot) Identity() string {
	if a.Kind == ActorUser {
		return a.UserID
	}
	return a.ID
}

func (a ActorSnapshot) Validate() error {
	if a.SchemaVersion != ActorSnapshotVersion {
		return serrors.NewInvalid("unsupported actor snapshot version")
	}
	switch a.Kind {
	case ActorUser:
		if a.UserID == "" || a.ID != "" {
			return serrors.NewInvalid("user actor requires user_id only")
		}
	case ActorService, ActorSystem:
		if a.ID == "" || a.DisplayName == "" || a.UserID != "" || a.FirstName != "" || a.LastName != "" || a.MiddleName != "" {
			return serrors.NewInvalid("service and system actors require id and display_name only")
		}
	default:
		return serrors.NewInvalid("unknown actor kind " + string(a.Kind))
	}
	return nil
}
