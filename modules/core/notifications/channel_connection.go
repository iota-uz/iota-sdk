package notifications

import (
	"context"

	"github.com/google/uuid"
)

// PlatformConnectionPrefix starts the connection ID of a platform credential:
// "platform:<kind>".
const PlatformConnectionPrefix = "platform:"

// ConnectorConnection is a working connection of a connector kind a channel
// can send through.
type ConnectorConnection struct {
	// ID is the connection UUID or "platform:<kind>".
	ID string
	// Name names the connection for people (the bot's @username, the
	// connection's display name).
	Name     string
	Platform bool
}

// ConnectorConnections lists a tenant's working connections of a connector
// kind: its own connections first, then the platform credential when it is
// enabled for the tenant. The automations module implements it.
type ConnectorConnections interface {
	Connections(ctx context.Context, tenantID uuid.UUID, kind string) ([]ConnectorConnection, error)
}

// ChannelConnectionRepository keeps the connection a tenant chose for a
// connector-backed channel.
type ChannelConnectionRepository interface {
	// Get returns the chosen connection; ok is false when the tenant chose none.
	Get(ctx context.Context, tenantID uuid.UUID, channel Channel) (connectionID string, ok bool, err error)
	// Set stores the choice; "" removes it.
	Set(ctx context.Context, tenantID uuid.UUID, channel Channel, connectionID string, by uint) error
}
