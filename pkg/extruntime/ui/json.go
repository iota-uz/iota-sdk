package ui

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/iota-uz/iota-sdk/pkg/composables"
)

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// RuntimeUser is the signed-in user as a client page may display it. It
// carries no authority: the server checks permissions on every call.
type RuntimeUser struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

func runtimeUser(ctx context.Context) *RuntimeUser {
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil {
		return nil
	}
	name := u.FirstName()
	if l := u.LastName(); l != "" {
		name += " " + l
	}
	return &RuntimeUser{ID: itoa(u.ID()), Name: name, Email: u.Email().Value()}
}

func itoa(n uint) string { return strconv.FormatUint(uint64(n), 10) }
