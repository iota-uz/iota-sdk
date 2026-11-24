package sms_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/sms"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/sms/smstest"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/eskizconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/twilioconfig"
)

const (
	eskizEmail    = "ops@example.com"
	eskizPassword = "s3cret-eskiz-pw"
	twilioSID     = "ACtestaccountxxxxxxxxxxxxxxxxxxxx"
	twilioToken   = "s3cretTwilioToken0123456789ab"
	twilioFrom    = "+15005550006"
)

type usage struct {
	mu      sync.Mutex
	entries []catalog.UsageEntry
}

func (u *usage) Record(_ context.Context, e catalog.UsageEntry) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.entries = append(u.entries, e)
	return nil
}

type statuses struct {
	mu   sync.Mutex
	sets []connectors.ConnectionStatus
	ids  []uuid.UUID
}

func (s *statuses) SetStatus(_ context.Context, _, id uuid.UUID, st connectors.ConnectionStatus, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sets, s.ids = append(s.sets, st), append(s.ids, id)
	return nil
}

// fixture is a connector over a fake provider and a call input for it.
type fixture struct {
	t     *testing.T
	node  catalog.CallNode
	conn  connectors.Connector
	in    catalog.CallInput
	usage *usage
	creds catalog.Credentials
	secs  []string
}

func newFixture(t *testing.T, kind catalog.ConnectorKind, opts ...sms.Option) (*fixture, *smstest.Eskiz, *smstest.Twilio) {
	t.Helper()
	f := &fixture{t: t, usage: &usage{}}
	var (
		esk *smstest.Eskiz
		twi *smstest.Twilio
	)
	switch kind {
	case sms.KindEskiz:
		esk = smstest.NewEskiz(eskizEmail, eskizPassword)
		t.Cleanup(esk.Close)
		f.creds = catalog.Credentials{"url": esk.URL(), "email": eskizEmail, "password": eskizPassword}
		f.secs = []string{eskizPassword}
		f.conn = sms.NewEskiz(append([]sms.Option{sms.WithTrustedEskizURLs(esk.URL())}, opts...)...)
	case sms.KindTwilio:
		twi = smstest.NewTwilio(twilioSID, twilioToken)
		t.Cleanup(twi.Close)
		f.creds = catalog.Credentials{"account_sid": twilioSID, "auth_token": twilioToken, "from": twilioFrom}
		f.secs = []string{twilioToken}
		f.conn = sms.NewTwilio(append([]sms.Option{sms.WithTransport(twi.Transport())}, opts...)...)
	}
	f.node = f.conn.Nodes()[0].(catalog.CallNode)
	f.in = catalog.CallInput{
		TenantID: uuid.New(), RunID: uuid.New(), NodeID: "send", Attempt: 1,
		Params:      map[string]any{"to": "+998 90 123-45-67", "text": "Hello"},
		Connection:  &catalog.ConnectionInfo{ID: uuid.NewString(), Kind: kind, Name: "main"},
		Credentials: f.creds, Usage: f.usage,
		Deadline: time.Now().Add(time.Minute),
	}
	return f, esk, twi
}

func (f *fixture) exec() (catalog.CallResult, error) {
	f.t.Helper()
	ctx, cancel := context.WithDeadline(context.Background(), f.in.Deadline)
	defer cancel()
	return f.node.Execute(ctx, f.in)
}

// runLikeEngine repeats the call after RetryAfter the way the engine does, and returns how many times
// it ran and when each began.
func (f *fixture) runLikeEngine() (catalog.CallResult, error, []time.Time) {
	f.t.Helper()
	var starts []time.Time
	for {
		starts = append(starts, time.Now())
		res, err := f.exec()
		if err != nil || res.RetryAfter == 0 {
			return res, err, starts
		}
		time.Sleep(res.RetryAfter)
		f.in.Attempt++
	}
}

func callErr(t *testing.T, err error) *catalog.CallError {
	t.Helper()
	var ce *catalog.CallError
	require.True(t, errors.As(err, &ce), "want a CallError, got %v", err)
	return ce
}

func (f *fixture) noSecrets(vals ...any) {
	f.t.Helper()
	for _, v := range vals {
		text := strings.ToLower(fmt.Sprintf("%v %+v", v, v))
		for _, sec := range f.secs {
			assert.NotContains(f.t, text, strings.ToLower(sec))
		}
	}
}

// ---- the nodes and the catalog

func TestNodesAndDescriptors(t *testing.T) {
	reg := catalog.NewRegistry(nil)
	dir := connectors.NewDirectory(reg)
	require.NoError(t, dir.Register(sms.NewEskiz()))
	require.NoError(t, dir.Register(sms.NewTwilio()))

	for kind, typ := range map[catalog.ConnectorKind]string{sms.KindEskiz: "eskiz.send_sms", sms.KindTwilio: "twilio.send_sms"} {
		c, ok := dir.Get(kind)
		require.True(t, ok)
		d := c.Descriptor()
		assert.Equal(t, connectors.PlatformOptIn, d.Platform, "the platform account is paid: opt in")
		assert.Equal(t, connectors.CategoryMessaging, d.Category)
		n, ok := reg.Node(typ)
		require.True(t, ok, typ)
		assert.Equal(t, graph.BehaviorCall, n.Spec().Behavior)
		assert.Equal(t, []catalog.ConnectorKind{kind}, n.Requires())
		assert.False(t, n.(catalog.CallNode).Idempotent(), "no idempotency key: a repeat may send twice")
		assert.Equal(t, []string{"sent", "failed"}, n.Spec().Outcomes)
	}

	e, _ := dir.Get(sms.KindEskiz)
	assert.Equal(t, map[string]bool{"password": true}, connectors.SecretFields(e.Descriptor()))
	tw, _ := dir.Get(sms.KindTwilio)
	assert.Equal(t, map[string]bool{"auth_token": true}, connectors.SecretFields(tw.Descriptor()))

	require.NoError(t, connectors.ValidateCredentials(e.Descriptor(), connectors.Credentials{"email": eskizEmail, "password": "x"}))
	require.Error(t, connectors.ValidateCredentials(e.Descriptor(), connectors.Credentials{"email": eskizEmail}))
	require.NoError(t, connectors.ValidateCredentials(tw.Descriptor(), connectors.Credentials{"account_sid": twilioSID, "auth_token": "t", "from": "+15005550006"}))
	require.NoError(t, connectors.ValidateCredentials(tw.Descriptor(), connectors.Credentials{"account_sid": twilioSID, "auth_token": "t", "from": "MG" + strings.Repeat("a", 32)}))
	require.Error(t, connectors.ValidateCredentials(tw.Descriptor(), connectors.Credentials{"account_sid": twilioSID, "auth_token": "t", "from": "15005550006"}))
	assert.NotContains(t, connectors.RedactedCredentials(e.Descriptor(), connectors.Credentials{"email": "a@b.co", "password": "p"}), "password")
}

type avail map[catalog.ConnectorKind][]catalog.ConnectionRef

func (a avail) Connected(context.Context, uuid.UUID) (map[catalog.ConnectorKind][]catalog.ConnectionRef, error) {
	return a, nil
}

func TestNodeIsInTheCatalogOnlyWithAConnection(t *testing.T) {
	ctx := context.Background()
	tenant := uuid.New()
	a := avail{}
	reg := catalog.NewRegistry(a)
	dir := connectors.NewDirectory(reg)
	require.NoError(t, dir.Register(sms.NewEskiz()))
	require.NoError(t, dir.Register(sms.NewTwilio()))

	v, err := reg.ForTenant(ctx, tenant)
	require.NoError(t, err)
	assert.NotContains(t, v.Catalog.Nodes, "twilio.send_sms")
	assert.Equal(t, catalog.ReasonConnectionUnavailable, v.Unavailable["twilio.send_sms"])

	g := graph.Graph{Nodes: []graph.Node{{ID: "send", Type: "twilio.send_sms", Params: map[string]any{"to": "+998901234567", "text": "x"}}}}
	errs := catalog.ConnectionRule(v)(g)
	require.Len(t, errs, 1)
	assert.Equal(t, catalog.CodeConnectionUnavailable, errs[0].Code)

	// An Eskiz connection does not make the Twilio node appear.
	a[sms.KindEskiz] = []catalog.ConnectionRef{{ID: uuid.NewString(), Name: "main"}}
	v, err = reg.ForTenant(ctx, tenant)
	require.NoError(t, err)
	assert.Contains(t, v.Catalog.Nodes, "eskiz.send_sms")
	assert.NotContains(t, v.Catalog.Nodes, "twilio.send_sms")

	// The platform credential makes it appear.
	a[sms.KindTwilio] = []catalog.ConnectionRef{{ID: catalog.PlatformConnectionID(sms.KindTwilio), Name: "platform"}}
	v, err = reg.ForTenant(ctx, tenant)
	require.NoError(t, err)
	assert.Contains(t, v.Catalog.Nodes, "twilio.send_sms")
	assert.Empty(t, catalog.ConnectionRule(v)(g))
}

func TestPlatformCredentialsFollowTheEnvironment(t *testing.T) {
	assert.Empty(t, sms.PlatformCredentials(nil, nil))
	assert.Empty(t, sms.PlatformCredentials(&eskizconfig.Config{Email: "a@b.co"}, &twilioconfig.Config{AccountSID: "AC1", AuthToken: "t"}),
		"the required fields enable a credential: Eskiz needs email and password, Twilio SID, token and number")

	got := sms.PlatformCredentials(
		&eskizconfig.Config{Email: eskizEmail, Password: eskizPassword, From: "4546"},
		&twilioconfig.Config{AccountSID: twilioSID, AuthToken: twilioToken, PhoneNumber: twilioFrom},
	)
	require.Len(t, got, 2)
	assert.Equal(t, eskizconfig.DefaultURL, got[sms.KindEskiz].Credentials["url"])
	assert.Equal(t, "4546", got[sms.KindEskiz].Credentials["from"])
	assert.Equal(t, twilioFrom, got[sms.KindTwilio].Credentials["from"])

	reg := catalog.NewRegistry(nil)
	dir := connectors.NewDirectory(reg)
	require.NoError(t, dir.Register(sms.NewEskiz()))
	require.NoError(t, dir.Register(sms.NewTwilio()))
	p := connectors.NewPlatformCredentials(dir, got).(connectors.PlatformCatalog)
	assert.ElementsMatch(t, []catalog.ConnectorKind{sms.KindEskiz, sms.KindTwilio}, p.Kinds())
	assert.Equal(t, connectors.PlatformOptIn, p.Policy(sms.KindTwilio))
}

// ---- Eskiz

func TestEskizSends(t *testing.T) {
	f, fake, _ := newFixture(t, sms.KindEskiz)
	f.creds["from"] = "4546"

	res, err := f.exec()
	require.NoError(t, err)
	assert.Equal(t, "sent", res.Outcome)
	assert.Equal(t, map[string]any{"provider": "eskiz", "provider_message_id": "eskiz-msg-1", "to": "+998901234567"}, res.Output)
	sends := fake.Sends()
	require.Len(t, sends, 1)
	assert.Equal(t, smstest.Sent{To: "998901234567", From: "4546", Text: "Hello", At: 1}, sends[0])

	require.Len(t, f.usage.entries, 1)
	u := f.usage.entries[0]
	assert.Equal(t, "sms", u.Unit)
	assert.EqualValues(t, 1, u.Quantity)
	assert.Equal(t, sms.KindEskiz, u.Kind)
	assert.Equal(t, f.in.Connection.ID, u.ConnectionID)
	assert.Equal(t, f.in.TenantID, u.TenantID)
	require.NotNil(t, u.RunID)
	assert.Equal(t, f.in.RunID, *u.RunID)

	// One client per connection: a second send does not log in again.
	_, err = f.exec()
	require.NoError(t, err)
	assert.Equal(t, 1, fake.Logins())

	// A token the provider no longer takes is renewed, but the refused SMS is not sent again.
	fake.ExpireTokens()
	before := fake.SendRequests()
	_, err = f.exec()
	require.Error(t, err)
	assert.Equal(t, catalog.CallUnavailable, callErr(t, err).Code, "not unauthorized: the credentials are good")
	assert.Equal(t, before+1, fake.SendRequests(), "the send endpoint got the SMS once")
	assert.Equal(t, 2, fake.Logins(), "the token was renewed")
	assert.Len(t, fake.Sends(), 2)

	res, err = f.exec()
	require.NoError(t, err)
	assert.Equal(t, "sent", res.Outcome)
	assert.Len(t, fake.Sends(), 3)
}

func TestEskizNewCredentialsGetANewClient(t *testing.T) {
	f, fake, _ := newFixture(t, sms.KindEskiz)
	_, err := f.exec()
	require.NoError(t, err)
	fake.Password = "changed"
	f.in.Credentials = catalog.Credentials{"url": fake.URL(), "email": eskizEmail, "password": "changed"}
	_, err = f.exec()
	require.NoError(t, err)
	assert.Equal(t, 2, fake.Logins())
}

func TestEskizOutcomeUnknownIsNotRepeated(t *testing.T) {
	f, fake, _ := newFixture(t, sms.KindEskiz)
	fake.Script(func(int) smstest.Reply { return smstest.Reply{Hang: true} })
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()

	res, err := f.node.Execute(ctx, f.in)
	require.Error(t, err)
	ce := callErr(t, err)
	assert.Equal(t, catalog.CallOutcomeUnknown, ce.Code)
	assert.False(t, ce.Retryable)
	assert.Zero(t, res.RetryAfter)
	assert.Len(t, fake.Sends(), 1, "the provider got the request once and is not asked again")
	assert.Empty(t, f.usage.entries, "nothing was recorded as sent")
	f.noSecrets(err)
}

func TestEskizRateLimitedIsRepeatedAfterRetryAfter(t *testing.T) {
	f, fake, _ := newFixture(t, sms.KindEskiz)
	fake.Script(func(n int) smstest.Reply {
		if n == 1 {
			return smstest.Reply{Status: 429, Header: map[string]string{"Retry-After": "2"}}
		}
		return smstest.Reply{}
	})
	res, err, starts := f.runLikeEngine()
	require.NoError(t, err)
	assert.Equal(t, "sent", res.Outcome)
	require.Len(t, starts, 2)
	assert.GreaterOrEqual(t, starts[1].Sub(starts[0]), 2*time.Second)
	assert.Len(t, fake.Sends(), 2)
	assert.Len(t, f.usage.entries, 1, "only the accepted message is counted")
}

func TestEskizRetryAfterIsReadFromTheAnswer(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		header string
		want   time.Duration
	}{
		"429 seconds": {429, "7", 7 * time.Second},
		"408 seconds": {408, "3", 3 * time.Second},
		"no header":   {429, "", time.Millisecond},
		"not a time":  {429, "soon", time.Millisecond},
	} {
		t.Run(name, func(t *testing.T) {
			f, fake, _ := newFixture(t, sms.KindEskiz)
			h := map[string]string{}
			if tc.header != "" {
				h["Retry-After"] = tc.header
			}
			fake.Script(func(int) smstest.Reply { return smstest.Reply{Status: tc.status, Header: h} })
			res, err := f.exec()
			require.NoError(t, err)
			assert.Empty(t, res.Outcome)
			assert.Equal(t, tc.want, res.RetryAfter)
		})
	}
}

func TestEskizErrors(t *testing.T) {
	cases := map[string]struct {
		reply smstest.Reply
		code  string
	}{
		"rejected":    {smstest.Reply{Status: 400, Body: `{"message":"Invalid mobile phone"}`}, catalog.CallRejected},
		"forbidden":   {smstest.Reply{Status: 403}, catalog.CallRejected},
		"unavailable": {smstest.Reply{Status: 503}, catalog.CallUnavailable},
		"unreadable":  {smstest.Reply{Status: 200, Body: `not json`}, catalog.CallOutcomeUnknown},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f, fake, _ := newFixture(t, sms.KindEskiz)
			fake.Script(func(int) smstest.Reply { return tc.reply })
			_, err := f.exec()
			require.Error(t, err)
			assert.Equal(t, tc.code, callErr(t, err).Code)
			assert.Len(t, fake.Sends(), 1)
			assert.Empty(t, f.usage.entries)
		})
	}
}

func TestEskizUnreachableIsUnavailable(t *testing.T) {
	f, fake, _ := newFixture(t, sms.KindEskiz)
	f.in.Credentials = catalog.Credentials{"url": fake.URL(), "email": eskizEmail, "password": eskizPassword}
	fake.Close() // nothing listens: the request never leaves
	_, err := f.exec()
	require.Error(t, err)
	assert.Equal(t, catalog.CallUnavailable, callErr(t, err).Code)
}

func TestEskizBadCredentialsAreUnauthorizedAndMarkTheConnection(t *testing.T) {
	st := &statuses{}
	f, fake, _ := newFixture(t, sms.KindEskiz, sms.WithConnectionStatus(st))
	f.in.Credentials = catalog.Credentials{"url": fake.URL(), "email": eskizEmail, "password": "wrong-password-xyz"}
	f.secs = append(f.secs, "wrong-password-xyz")

	res, err := f.exec()
	require.Error(t, err)
	assert.Equal(t, catalog.CallUnauthorized, callErr(t, err).Code)
	assert.Empty(t, fake.Sends())
	require.Equal(t, []connectors.ConnectionStatus{connectors.StatusNeedsReauth}, st.sets)
	assert.Equal(t, f.in.Connection.ID, st.ids[0].String())
	f.noSecrets(err, res)

	// The platform credential has no row to mark.
	st2 := &statuses{}
	f2, fake2, _ := newFixture(t, sms.KindEskiz, sms.WithConnectionStatus(st2))
	f2.in.Credentials = catalog.Credentials{"url": fake2.URL(), "email": eskizEmail, "password": "wrong"}
	f2.in.Connection = &catalog.ConnectionInfo{ID: "platform:eskiz", Kind: sms.KindEskiz, Platform: true}
	_, err = f2.exec()
	assert.Equal(t, catalog.CallUnauthorized, callErr(t, err).Code)
	assert.Empty(t, st2.sets)
}

func TestInvalidRequestsAreRefusedBeforeAnyRequest(t *testing.T) {
	for _, kind := range []catalog.ConnectorKind{sms.KindEskiz, sms.KindTwilio} {
		t.Run(string(kind), func(t *testing.T) {
			f, esk, twi := newFixture(t, kind)
			sent := func() int {
				if esk != nil {
					return len(esk.Sends())
				}
				return len(twi.Sends())
			}
			for name, params := range map[string]map[string]any{
				"no country code": {"to": "901234567", "text": "x"},
				"not a number":    {"to": "call me", "text": "x"},
				"empty text":      {"to": "+998901234567", "text": "   "},
			} {
				f.in.Params = params
				_, err := f.exec()
				require.Error(t, err, name)
				ce := callErr(t, err)
				assert.Equal(t, catalog.CallInvalidRequest, ce.Code, name)
				assert.NotEmpty(t, ce.Message, name)
			}
			assert.Zero(t, sent())
		})
	}
	f, _, _ := newFixture(t, sms.KindEskiz, sms.WithMaxMessageSize(10))
	f.in.Params = map[string]any{"to": "+998901234567", "text": "Привет, мир!!"} // 13 characters, 24 bytes
	_, err := f.exec()
	assert.Equal(t, catalog.CallInvalidRequest, callErr(t, err).Code)
	f.in.Params["text"] = "Привет мир" // 10 characters
	res, err := f.exec()
	require.NoError(t, err, "the limit counts characters, not bytes")
	assert.Equal(t, "sent", res.Outcome)
}

func TestDefaultCountryCode(t *testing.T) {
	f, fake, _ := newFixture(t, sms.KindEskiz, sms.WithDefaultCountryCode("+998"))
	f.in.Params = map[string]any{"to": "90 123 45 67", "text": "x"}
	res, err := f.exec()
	require.NoError(t, err)
	assert.Equal(t, "+998901234567", res.Output["to"])
	assert.Equal(t, "998901234567", fake.Sends()[0].To)
}

func TestEskizTest(t *testing.T) {
	f, fake, _ := newFixture(t, sms.KindEskiz)
	id, err := f.conn.Test(context.Background(), connectors.Credentials(f.creds))
	require.NoError(t, err)
	assert.Equal(t, connectors.Identity{Display: eskizEmail, External: eskizEmail, Kind: connectors.IdentityEmail}, id)
	assert.Empty(t, fake.Sends(), "the check sends nothing")

	_, err = f.conn.Test(context.Background(), connectors.Credentials{"url": fake.URL(), "email": eskizEmail, "password": "nope-nope"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "nope-nope")
	assert.Contains(t, err.Error(), "refused the credentials")
}

// ---- Twilio

func TestTwilioSends(t *testing.T) {
	f, _, fake := newFixture(t, sms.KindTwilio)
	res, err := f.exec()
	require.NoError(t, err)
	assert.Equal(t, "sent", res.Outcome)
	assert.Equal(t, "twilio", res.Output["provider"])
	assert.Equal(t, "+998901234567", res.Output["to"])
	assert.Equal(t, "SM000000000000000000000000000001", res.Output["provider_message_id"])
	assert.EqualValues(t, 2, res.Output["segments"])
	assert.Equal(t, []smstest.Sent{{To: "+998901234567", From: twilioFrom, Text: "Hello", At: 1}}, fake.Sends())

	require.Len(t, f.usage.entries, 1)
	assert.Equal(t, "sms", f.usage.entries[0].Unit)
	assert.EqualValues(t, 2, f.usage.entries[0].Quantity, "a message of two segments counts twice")
}

func TestTwilioMessagingService(t *testing.T) {
	f, _, fake := newFixture(t, sms.KindTwilio)
	mg := "MG" + strings.Repeat("0", 32)
	f.in.Credentials = catalog.Credentials{"account_sid": twilioSID, "auth_token": twilioToken, "from": mg}
	_, err := f.exec()
	require.NoError(t, err)
	got := fake.Sends()[0]
	assert.Equal(t, mg, got.Service)
	assert.Empty(t, got.From)

	f.in.Credentials["from"] = "not a sender"
	_, err = f.exec()
	assert.Equal(t, catalog.CallInvalidRequest, callErr(t, err).Code)
	assert.Len(t, fake.Sends(), 1)
}

func TestTwilioOutcomeUnknownIsNotRepeated(t *testing.T) {
	f, _, fake := newFixture(t, sms.KindTwilio)
	fake.Script(func(int) smstest.Reply { return smstest.Reply{Hang: true} })
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	_, err := f.node.Execute(ctx, f.in)
	require.Error(t, err)
	assert.Equal(t, catalog.CallOutcomeUnknown, callErr(t, err).Code)
	assert.Len(t, fake.Sends(), 1)
	f.noSecrets(err)
}

func TestTwilioRateLimitedIsRepeated(t *testing.T) {
	f, _, fake := newFixture(t, sms.KindTwilio)
	fake.Script(func(n int) smstest.Reply {
		if n == 1 {
			return smstest.Reply{Status: 429, Header: map[string]string{"Retry-After": "1"}, Body: `{"code":20429,"message":"Too many requests","status":429}`}
		}
		return smstest.Reply{}
	})
	res, err, starts := f.runLikeEngine()
	require.NoError(t, err)
	assert.Equal(t, "sent", res.Outcome)
	require.Len(t, starts, 2)
	assert.GreaterOrEqual(t, starts[1].Sub(starts[0]), time.Second)
	assert.Len(t, fake.Sends(), 2)
}

func TestTwilioErrors(t *testing.T) {
	cases := map[string]struct {
		reply smstest.Reply
		code  string
	}{
		"rejected":    {smstest.Reply{Status: 400, Body: `{"code":21211,"message":"The 'To' number is invalid","status":400}`}, catalog.CallRejected},
		"unavailable": {smstest.Reply{Status: 502}, catalog.CallUnavailable},
		"unreadable":  {smstest.Reply{Status: 201, Body: `not json`}, catalog.CallOutcomeUnknown},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f, _, fake := newFixture(t, sms.KindTwilio)
			fake.Script(func(int) smstest.Reply { return tc.reply })
			_, err := f.exec()
			require.Error(t, err)
			ce := callErr(t, err)
			assert.Equal(t, tc.code, ce.Code)
			if name == "rejected" {
				assert.Contains(t, ce.Message, "The 'To' number is invalid", "the provider's reason is kept")
			}
			assert.Len(t, fake.Sends(), 1)
		})
	}
}

func TestTwilioBadCredentials(t *testing.T) {
	st := &statuses{}
	f, _, fake := newFixture(t, sms.KindTwilio, sms.WithConnectionStatus(st))
	f.in.Credentials = catalog.Credentials{"account_sid": twilioSID, "auth_token": "wrongTokenXyz0123", "from": twilioFrom}
	f.secs = append(f.secs, "wrongTokenXyz0123")
	res, err := f.exec()
	require.Error(t, err)
	assert.Equal(t, catalog.CallUnauthorized, callErr(t, err).Code)
	assert.Equal(t, []connectors.ConnectionStatus{connectors.StatusNeedsReauth}, st.sets)
	assert.Empty(t, fake.Sends())
	f.noSecrets(err, res)
}

func TestTwilioTest(t *testing.T) {
	f, _, fake := newFixture(t, sms.KindTwilio)
	id, err := f.conn.Test(context.Background(), connectors.Credentials(f.creds))
	require.NoError(t, err)
	assert.Equal(t, connectors.Identity{Display: twilioFrom, External: twilioSID + "/" + twilioFrom, Kind: connectors.IdentityPhone}, id)
	assert.Empty(t, fake.Sends())
	assert.NotContains(t, id.Display+id.External, twilioToken)

	id, err = f.conn.Test(context.Background(), connectors.Credentials{"account_sid": twilioSID, "auth_token": twilioToken, "from": "MG" + strings.Repeat("1", 32)})
	require.NoError(t, err)
	assert.Empty(t, id.Display, "a Messaging Service SID is not a public identity")
	assert.Equal(t, twilioSID+"/MG"+strings.Repeat("1", 32), id.External, "the sender is kept")

	_, err = f.conn.Test(context.Background(), connectors.Credentials{"account_sid": twilioSID, "auth_token": "bad-token-1", "from": twilioFrom})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "bad-token-1")
}

func TestNoCredentials(t *testing.T) {
	for _, kind := range []catalog.ConnectorKind{sms.KindEskiz, sms.KindTwilio} {
		f, _, _ := newFixture(t, kind)
		f.in.Credentials = nil
		_, err := f.exec()
		assert.Equal(t, catalog.CallUnauthorized, callErr(t, err).Code)
	}
}

func TestEskizURLPolicy(t *testing.T) {
	f, fake, _ := newFixture(t, sms.KindEskiz)
	for _, u := range []string{"http://127.0.0.1:9", "https://169.254.169.254", "https://notify.eskiz.uz.evil.example", "https://user@notify.eskiz.uz", "https://notify.eskiz.uz/x"} {
		f.in.Credentials = catalog.Credentials{"url": u, "email": eskizEmail, "password": eskizPassword}
		_, err := f.exec()
		require.Error(t, err, u)
		assert.Equal(t, catalog.CallInvalidRequest, callErr(t, err).Code, u)
		_, err = f.conn.Test(context.Background(), connectors.Credentials(f.in.Credentials))
		require.Error(t, err, u)
	}
	// The platform credential's URL comes from the environment and is trusted.
	f.in.Credentials = f.creds
	f.in.Connection = &catalog.ConnectionInfo{ID: "platform:eskiz", Kind: sms.KindEskiz, Platform: true}
	other := sms.NewEskiz().Nodes()[0].(catalog.CallNode)
	res, err := other.Execute(context.Background(), f.in)
	require.NoError(t, err)
	assert.Equal(t, "sent", res.Outcome)
	assert.Len(t, fake.Sends(), 1)
}

func TestRedirectIsNotFollowed(t *testing.T) {
	f, fake, _ := newFixture(t, sms.KindEskiz)
	target := smstest.NewEskiz(eskizEmail, eskizPassword)
	t.Cleanup(target.Close)
	fake.Script(func(int) smstest.Reply {
		return smstest.Reply{Status: 307, Header: map[string]string{"Location": target.URL() + "/api/message/sms/send"}}
	})
	_, err := f.exec()
	require.Error(t, err)
	assert.Equal(t, catalog.CallOutcomeUnknown, callErr(t, err).Code)
	assert.Len(t, fake.Sends(), 1)
	assert.Empty(t, target.Sends(), "the SMS is not replayed at the redirect target")
}

func TestAnswerWithoutAnIDIsNotSent(t *testing.T) {
	for _, kind := range []catalog.ConnectorKind{sms.KindEskiz, sms.KindTwilio} {
		f, esk, twi := newFixture(t, kind)
		body := `{}`
		status := 200
		script := func(int) smstest.Reply { return smstest.Reply{Status: status, Body: body} }
		if esk != nil {
			esk.Script(script)
		} else {
			status = 201
			twi.Script(script)
		}
		_, err := f.exec()
		require.Error(t, err, kind)
		assert.Equal(t, catalog.CallOutcomeUnknown, callErr(t, err).Code, kind)
		assert.Empty(t, f.usage.entries, kind)
	}
}

func TestUnknownOutcomeIsTerminal(t *testing.T) {
	for _, c := range []connectors.Connector{sms.NewEskiz(), sms.NewTwilio()} {
		tu, ok := c.Nodes()[0].(catalog.TerminalUnknown)
		require.True(t, ok)
		assert.True(t, tu.UnknownIsTerminal())
	}
}
