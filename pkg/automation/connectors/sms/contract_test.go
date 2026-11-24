package sms_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/sms"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/sms/smstest"
)

// The contract test runs the same scenarios against the fake providers (CONTRACT_TARGET=fake, the
// default) or the real ones (CONTRACT_TARGET=live, a manual run with an account):
//
//	CONTRACT_TARGET=live ESKIZ_EMAIL=… ESKIZ_PASSWORD=… ESKIZ_TEST_TO=+998… \
//	TWILIO_ACCOUNTSID=… TWILIO_AUTHTOKEN=… TWILIO_PHONENUMBER=… TWILIO_TEST_TO=+… \
//	go test ./pkg/automation/connectors/sms -run TestContract -v
//
// A provider whose variables are not set is skipped in live mode. An Eskiz account on test status sends
// only its fixed test texts, so the live text is one of them.
const eskizLiveText = "Это тест от Eskiz"

type target struct {
	kind  catalog.ConnectorKind
	conn  connectors.Connector
	creds catalog.Credentials
	// bad is credentials the provider refuses.
	bad  catalog.Credentials
	to   string
	text string
}

func contractTargets(t *testing.T) []target {
	t.Helper()
	switch mode := os.Getenv("CONTRACT_TARGET"); mode {
	case "", "fake":
		esk := smstest.NewEskiz(eskizEmail, eskizPassword)
		t.Cleanup(esk.Close)
		twi := smstest.NewTwilio(twilioSID, twilioToken)
		t.Cleanup(twi.Close)
		return []target{
			{sms.KindEskiz, sms.NewEskiz(sms.WithTrustedEskizURLs(esk.URL())),
				catalog.Credentials{"url": esk.URL(), "email": eskizEmail, "password": eskizPassword},
				catalog.Credentials{"url": esk.URL(), "email": eskizEmail, "password": "badpassword"},
				"+998901234567", "Hello"},
			{sms.KindTwilio, sms.NewTwilio(sms.WithTransport(twi.Transport())),
				catalog.Credentials{"account_sid": twilioSID, "auth_token": twilioToken, "from": twilioFrom},
				catalog.Credentials{"account_sid": twilioSID, "auth_token": "badToken0123456789", "from": twilioFrom},
				"+998901234567", "Hello"},
		}
	case "live":
		var out []target
		if email, pw, to := os.Getenv("ESKIZ_EMAIL"), os.Getenv("ESKIZ_PASSWORD"), os.Getenv("ESKIZ_TEST_TO"); email != "" && pw != "" && to != "" {
			creds := catalog.Credentials{"email": email, "password": pw, "from": os.Getenv("ESKIZ_FROM")}
			if u := os.Getenv("ESKIZ_URL"); u != "" {
				creds["url"] = u
			}
			bad := catalog.Credentials{"email": email, "password": pw + "x"}
			out = append(out, target{sms.KindEskiz, sms.NewEskiz(), creds, bad, to, eskizLiveText})
		}
		if sid, tok, from, to := os.Getenv("TWILIO_ACCOUNTSID"), os.Getenv("TWILIO_AUTHTOKEN"), os.Getenv("TWILIO_PHONENUMBER"), os.Getenv("TWILIO_TEST_TO"); sid != "" && tok != "" && from != "" && to != "" {
			creds := catalog.Credentials{"account_sid": sid, "auth_token": tok, "from": from}
			bad := catalog.Credentials{"account_sid": sid, "auth_token": tok + "x", "from": from}
			out = append(out, target{sms.KindTwilio, sms.NewTwilio(), creds, bad, to, "Contract test"})
		}
		if len(out) == 0 {
			t.Skip("CONTRACT_TARGET=live: no provider credentials in the environment")
		}
		return out
	default:
		t.Fatalf("CONTRACT_TARGET is fake or live, got %q", mode)
		return nil
	}
}

func TestContract(t *testing.T) {
	for _, tg := range contractTargets(t) {
		t.Run(string(tg.kind), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			node := tg.conn.Nodes()[0].(catalog.CallNode)
			in := func(creds catalog.Credentials, to, text string) catalog.CallInput {
				return catalog.CallInput{
					TenantID: uuid.New(), RunID: uuid.New(), NodeID: "send", Attempt: 1,
					Params:      map[string]any{"to": to, "text": text},
					Connection:  &catalog.ConnectionInfo{ID: uuid.NewString(), Kind: tg.kind},
					Credentials: creds, Deadline: time.Now().Add(time.Minute),
				}
			}

			id, err := tg.conn.Test(ctx, connectors.Credentials(tg.creds))
			require.NoError(t, err, "valid credentials pass the check")
			assert.NotEmpty(t, id.External)

			_, err = tg.conn.Test(ctx, connectors.Credentials(tg.bad))
			require.Error(t, err, "refused credentials fail the check")
			for _, v := range tg.bad {
				if v == tg.bad["password"] || v == tg.bad["auth_token"] {
					assert.NotContains(t, err.Error(), v)
				}
			}

			res, err := node.Execute(ctx, in(tg.creds, tg.to, tg.text))
			require.NoError(t, err)
			assert.Equal(t, "sent", res.Outcome)
			assert.Equal(t, string(tg.kind), res.Output["provider"])
			assert.NotEmpty(t, res.Output["provider_message_id"])
			assert.Equal(t, tg.to, res.Output["to"])

			_, err = node.Execute(ctx, in(tg.creds, "not a number", tg.text))
			require.Error(t, err)
			assert.Equal(t, catalog.CallInvalidRequest, callErr(t, err).Code)

			_, err = node.Execute(ctx, in(tg.bad, tg.to, tg.text))
			require.Error(t, err)
			assert.Equal(t, catalog.CallUnauthorized, callErr(t, err).Code)
		})
	}
}
