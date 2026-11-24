package email

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

// accessToken is the token an OAuth account authenticates with: a valid one, refreshed by the OAuth2 of
// the application. A refusal of the refresh (the access was revoked) marks both connections of the sign-in
// needs_reauth and is returned as connectors.ErrNeedsReauth.
func (h *hub) accessToken(ctx context.Context, tenant uuid.UUID, connectionID string, creds connectors.Credentials) (string, error) {
	id, err := uuid.Parse(connectionID)
	if err != nil || h.cfg.OAuth == nil {
		// No connection to refresh through (a test before saving): what the credentials hold.
		return tokenOf(creds)
	}
	tok, err := h.cfg.OAuth.Token(ctx, tenant, id)
	switch {
	case err == nil:
		return tok, nil
	case errors.Is(err, connectors.ErrNeedsReauth):
		h.markLogin(ctx, tenant, id, creds)
		return "", connectors.ErrNeedsReauth
	}
	return "", fmt.Errorf("email: the access token could not be refreshed: %w", errors.New("token request failed"))
}

// markLogin marks the other connections of the same sign-in (same login id) needs_reauth: the token they
// share was revoked, so none of them can work.
func (h *hub) markLogin(ctx context.Context, tenant, self uuid.UUID, creds connectors.Credentials) {
	login := creds["login_id"]
	if login == "" || h.cfg.Connections == nil {
		return
	}
	for _, kind := range []catalog.ConnectorKind{KindSMTP, KindIMAP} {
		list, err := h.cfg.Connections.List(ctx, tenant, kind)
		if err != nil {
			h.log().Warn("email: connections of a sign-in could not be listed", "error", err.Error())
			continue
		}
		for _, c := range list {
			if c.ID == self || c.Status == connectors.StatusNeedsReauth {
				continue
			}
			_, cc, err := h.cfg.Connections.Get(ctx, tenant, c.ID)
			if err != nil || cc["login_id"] != login {
				continue
			}
			if err := h.cfg.Connections.SetStatus(ctx, tenant, c.ID, connectors.StatusNeedsReauth, "the provider refused the refresh token"); err != nil {
				h.log().Warn("email: a connection of a sign-in could not be marked", "error", err.Error())
			}
		}
	}
}

// callToken is accessToken for a node: a revoked sign-in is a call that failed unauthorized.
func (h *hub) callToken(ctx context.Context, in catalog.CallInput, a account) (string, error) {
	if !a.oauth() {
		return "", nil
	}
	id := ""
	if in.Connection != nil {
		id = in.Connection.ID
	}
	tok, err := h.accessToken(ctx, in.TenantID, id, in.Credentials)
	if errors.Is(err, connectors.ErrNeedsReauth) {
		return "", &catalog.CallError{Code: catalog.CallUnauthorized, Message: "the sign-in was revoked: connect the mailbox again"}
	}
	if err != nil {
		return "", &catalog.CallError{Code: catalog.CallUnavailable, Message: "the access token could not be refreshed", Retryable: true}
	}
	return tok, nil
}
