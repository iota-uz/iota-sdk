// Package realtime defines backend-neutral targeted realtime fanout.
package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const MaxPayloadBytes = 64 * 1024

var ErrOversize = errors.New("realtime envelope exceeds 64 KiB")

type Envelope struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	Channel     string    `json:"channel"`
	Payload     []byte    `json:"payload"`
	PublishedAt time.Time `json:"published_at"`
}
type Handler func(context.Context, Envelope) error
type Backend interface {
	Publish(context.Context, Envelope) error
	Subscribe(context.Context, Handler) error
	Close() error
}

func UserChannel(tenant uuid.UUID, userID uint) string {
	return fmt.Sprintf("tenant/%s/user/%d", tenant, userID)
}
func (e Envelope) Validate() error {
	if len(e.Payload)+len(e.Channel)+128 > MaxPayloadBytes {
		return ErrOversize
	}
	if e.ID == uuid.Nil || e.TenantID == uuid.Nil || e.PublishedAt.IsZero() || !strings.HasPrefix(e.Channel, "tenant/"+e.TenantID.String()+"/") {
		return errors.New("invalid realtime envelope")
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if len(data) > MaxPayloadBytes {
		return ErrOversize
	}
	return nil
}

type Counters struct {
	Published       uint64
	Received        uint64
	Duplicate       uint64
	Reconnect       uint64
	Oversize        uint64
	DeliveryFailure uint64
}
type Observable interface{ Counters() Counters }
