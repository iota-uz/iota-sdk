package email

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email/emailtest"
)

// The contract test: what the connector relies on a mail server for, run against the in-memory fake
// (CONTRACT_TARGET=fake, the default) and, by hand, against a real mailbox:
//
//	CONTRACT_TARGET=live \
//	EMAIL_CONTRACT_SMTP_HOST=smtp.example.com EMAIL_CONTRACT_SMTP_PORT=587 \
//	EMAIL_CONTRACT_IMAP_HOST=imap.example.com EMAIL_CONTRACT_IMAP_PORT=993 \
//	EMAIL_CONTRACT_USER=me@example.com EMAIL_CONTRACT_PASSWORD=<app password> \
//	  go test ./pkg/automation/connectors/email -run TestMailServerContract -v
//
// The live run sends a few messages from the mailbox to itself, reads them from INBOX and marks them read;
// it deletes nothing. It waits up to a minute for the mail to arrive. It is not part of CI.
type contract struct {
	smtp, imap connectors.Credentials
	// deliver is how a message sent to the mailbox shows up in it: nothing for a real server, which
	// delivers it itself; the fake keeps what it was sent and files it.
	settle func()
	live   bool
	user   string
	srv    *emailtest.Server
}

func contractTarget(t *testing.T) (*contract, *fx) {
	t.Helper()
	switch mode := os.Getenv("CONTRACT_TARGET"); mode {
	case "", "fake":
		f := newFx(t, emailtest.ModeSTARTTLS)
		c := &contract{smtp: f.smtpCreds(), imap: f.imapCreds(), user: mailbox, srv: f.srv}
		c.settle = func() {
			// The fake SMTP server keeps what it took; a real one hands it to the mailbox.
			for _, s := range f.srv.Sent() {
				f.srv.Deliver(mailbox, "INBOX", s.Data)
			}
		}
		return c, f
	case "live":
		env := func(k string) string { return os.Getenv("EMAIL_CONTRACT_" + k) }
		for _, k := range []string{"SMTP_HOST", "IMAP_HOST", "USER", "PASSWORD"} {
			if env(k) == "" {
				t.Skip("CONTRACT_TARGET=live needs EMAIL_CONTRACT_SMTP_HOST, _IMAP_HOST, _USER and _PASSWORD")
			}
		}
		or := func(v, d string) string {
			if v == "" {
				return d
			}
			return v
		}
		f := &fx{uploads: &memUploads{}, usage: &usage{}, oauth: &fakeOAuth{}, conns: &fakeConns{}}
		cs := New(Config{Uploads: f.uploads})
		f.smtp, f.imap = cs[0].(*smtpConnector), cs[1].(*imapConnector)
		return &contract{live: true, user: env("USER"),
			smtp:   connectors.Credentials{"preset": "custom", "host": env("SMTP_HOST"), "port": or(env("SMTP_PORT"), "587"), "username": env("USER"), "password": env("PASSWORD"), "from_address": env("USER")},
			imap:   connectors.Credentials{"preset": "custom", "host": env("IMAP_HOST"), "port": or(env("IMAP_PORT"), "993"), "username": env("USER"), "password": env("PASSWORD")},
			settle: func() { time.Sleep(20 * time.Second) },
		}, f
	default:
		t.Fatalf("CONTRACT_TARGET is fake or live, not %q", mode)
		return nil, nil
	}
}

func TestMailServerContract(t *testing.T) {
	c, f := contractTarget(t)
	ctx := context.Background()

	t.Run("a login is checked without sending a message", func(t *testing.T) {
		id, err := f.smtp.Test(ctx, c.smtp)
		require.NoError(t, err)
		assert.Equal(t, strings.ToLower(c.user), id.External)
		id, err = f.imap.Test(ctx, c.imap)
		require.NoError(t, err)
		assert.Equal(t, connectors.IdentityEmail, id.Kind)
		if !c.live {
			assert.Empty(t, c.srv.Sent())
		}
	})
	t.Run("a refused login is unauthorized", func(t *testing.T) {
		bad := connectors.Credentials{}
		for k, v := range c.smtp {
			bad[k] = v
		}
		bad["password"] = "definitely-not-the-password"
		_, err := f.smtp.Test(ctx, bad)
		require.Error(t, err)
		assert.Equal(t, catalog.CallUnauthorized, callError(t, err).Code)
		bad = connectors.Credentials{}
		for k, v := range c.imap {
			bad[k] = v
		}
		bad["password"] = "definitely-not-the-password"
		_, err = f.imap.Test(ctx, bad)
		assert.Equal(t, catalog.CallUnauthorized, callError(t, err).Code)
	})
	t.Run("a message sent to the mailbox is polled, found by its Message-ID and marked", func(t *testing.T) {
		conn := connectors.Connection{ID: uuid.New(), TenantID: tenant, Kind: KindIMAP, Identity: connectors.Identity{External: strings.ToLower(c.user)}}
		_, cur, err := f.imap.Poll(ctx, conn, c.imap, nil)
		require.NoError(t, err)

		in := f.sendInput(map[string]any{"to": []any{c.user}, "subject": "contract " + uuid.NewString(), "text": "hello"}, c.smtp)
		res, err := (&sendNode{f.smtp.hub}).Execute(ctx, in)
		require.NoError(t, err)
		id := res.Output["message_id"].(string)
		c.settle()

		var evs int
		deadline := time.Now().Add(time.Minute)
		for time.Now().Before(deadline) {
			events, next, err := f.imap.Poll(ctx, conn, c.imap, cur)
			require.NoError(t, err)
			cur = next
			for _, ev := range events {
				if ev.DedupKey == id {
					evs++
					assert.Equal(t, conn.ID.String()+":"+id, ev.Subject.Key, "a message with no references is its own thread")
				}
			}
			if evs > 0 || !c.live {
				break
			}
			time.Sleep(5 * time.Second)
		}
		require.Equal(t, 1, evs, "the message sent to the mailbox arrives as an event, once")

		label, err := (&labelNode{f.imap.hub}).Execute(ctx, in2(c.imap, map[string]any{"message_id": id, "mark_seen": true}))
		require.NoError(t, err)
		assert.Equal(t, "done", label.Outcome)
	})
}

func in2(creds connectors.Credentials, params map[string]any) catalog.CallInput {
	return catalog.CallInput{TenantID: tenant, Params: params, Credentials: creds, Connection: &catalog.ConnectionInfo{ID: uuid.NewString(), Kind: KindIMAP}}
}
