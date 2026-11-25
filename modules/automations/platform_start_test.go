package automations_test

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// bot is a connector with an exclusive account and a platform credential it sets up at the start.
type bot struct {
	mu      sync.Mutex
	started []string
}

func (*bot) Kind() catalog.ConnectorKind       { return catalog.KindTelegramBot }
func (*bot) Nodes() []catalog.NodeType         { return nil }
func (*bot) Triggers() []catalog.TriggerSource { return nil }
func (*bot) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{Kind: catalog.KindTelegramBot}
}
func (*bot) ExternalID(c connectors.Credentials) string { return "acct-" + c["token"] }
func (*bot) Test(context.Context, connectors.Credentials) (connectors.Identity, error) {
	return connectors.Identity{}, nil
}
func (b *bot) StartPlatform(_ context.Context, c connectors.Credentials) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.started = append(b.started, c["token"])
	return nil
}
func (b *bot) startedWith() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.started...)
}

type directory struct{ list []connectors.Connector }

func (d directory) Register(connectors.Connector) error { return nil }
func (d directory) List() []connectors.Connector        { return d.list }
func (d directory) Get(kind catalog.ConnectorKind) (connectors.Connector, bool) {
	for _, c := range d.list {
		if c.Kind() == kind {
			return c, true
		}
	}
	return nil, false
}

// pinned is a platform credential set that cannot be withdrawn: only the read side of PlatformCredentials.
type pinned struct{ connectors.PlatformCredentials }

type logged struct {
	mu   sync.Mutex
	logs []string
}

func (l *logged) warn(kind catalog.ConnectorKind, format string, _ error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, string(kind)+": "+format)
}

func startEnv(t *testing.T) (*pgxpool.Pool, uuid.UUID, *bot, connectors.Directory, map[catalog.ConnectorKind]connectors.PlatformCredential) {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automations_platform_start",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	b := &bot{}
	creds := map[catalog.ConnectorKind]connectors.PlatformCredential{catalog.KindTelegramBot: {Credentials: connectors.Credentials{"token": "platform"}}}
	return scope.Pool, scope.Tenant.ID, b, directory{list: []connectors.Connector{b}}, creds
}

// holdAccount saves a tenant's connection and reserves the platform credential's account for it.
func holdAccount(t *testing.T, db *pgxpool.Pool, tenant uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": bytes.Repeat([]byte{7}, 32)})
	require.NoError(t, err)
	store := connectors.NewStore(db, keys, nil, nil, nil)
	held, err := store.Save(ctx, connectors.Connection{TenantID: tenant, Kind: catalog.KindTelegramBot, Name: "held", DisplayName: "Held"}, connectors.Credentials{"token": "platform"})
	require.NoError(t, err)
	require.NoError(t, pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		return connectors.ReserveIdentity(ctx, tx, catalog.KindTelegramBot, "acct-platform", held.ID)
	}))
	return held.ID
}

// A tenant's connection holds the account of the platform credential: the credential is withdrawn (it is served to
// nobody) and its webhook is not set, so the tenant's webhook is not replaced.
func TestAPlatformCredentialOfAHeldAccountIsWithdrawn(t *testing.T) {
	db, tenant, b, dir, creds := startEnv(t)
	ctx := context.Background()
	holdAccount(t, db, tenant)
	platform := connectors.NewPlatformCredentials(dir, creds)
	log := &logged{}
	require.NoError(t, automations.StartPlatformConnectors(ctx, dir, platform, db, nil, log.warn))
	_, _, has := platform.Platform(catalog.KindTelegramBot)
	assert.False(t, has, "the withdrawn credential is not served")
	assert.Empty(t, b.startedWith(), "and its webhook is not set")
	assert.NotEmpty(t, log.logs)
}

// A credential of an exclusive account that cannot be withdrawn is never started: its reservation is a lease, and
// once a tenant's connection took the account it could not stop being served. Free or held, the start fails.
func TestAPlatformCredentialThatCannotBeWithdrawnStopsTheStart(t *testing.T) {
	db, tenant, b, dir, creds := startEnv(t)
	ctx := context.Background()
	platform := pinned{connectors.NewPlatformCredentials(dir, creds)}
	err := automations.StartPlatformConnectors(ctx, dir, platform, db, nil, (&logged{}).warn)
	require.ErrorIs(t, err, automations.ErrPlatformNotWithdrawable, "a free account too")
	holdAccount(t, db, tenant)
	err = automations.StartPlatformConnectors(ctx, dir, platform, db, nil, (&logged{}).warn)
	require.ErrorIs(t, err, automations.ErrPlatformNotWithdrawable)
	assert.Empty(t, b.startedWith())
}

// A reservation that cannot be checked (the database fails) is no reason to withdraw the credential: no webhook is
// set, the credential keeps sending, and the renewal reserves it once the database answers.
func TestATransientReservationFailureKeepsTheCredential(t *testing.T) {
	db, _, b, dir, creds := startEnv(t)
	ctx := context.Background()
	broken, err := pgxpool.New(ctx, db.Config().ConnString())
	require.NoError(t, err)
	broken.Close()
	platform := connectors.NewPlatformCredentials(dir, creds)
	log := &logged{}
	require.NoError(t, automations.StartPlatformConnectors(ctx, dir, platform, broken, nil, log.warn))
	assert.Empty(t, b.startedWith(), "no webhook without the reservation")
	_, _, has := platform.Platform(catalog.KindTelegramBot)
	assert.True(t, has, "the credential still sends")
	assert.NotEmpty(t, log.logs)

	automations.RenewPlatformConnectors(ctx, dir, platform, db, log.warn)
	var held int
	require.NoError(t, db.QueryRow(ctx, `SELECT count(*) FROM automation.exclusive_identities WHERE platform AND external = 'acct-platform'`).Scan(&held))
	assert.Equal(t, 1, held, "the renewal reserves it once the database answers")
}

// A free account is reserved for the platform and its webhook is set; with no database to check the reservation the
// webhook is not set (the credential still sends).
func TestAPlatformCredentialIsStartedOnlyWithItsReservation(t *testing.T) {
	db, _, b, dir, creds := startEnv(t)
	ctx := context.Background()
	platform := connectors.NewPlatformCredentials(dir, creds)
	require.NoError(t, automations.StartPlatformConnectors(ctx, dir, platform, db, nil, (&logged{}).warn))
	assert.Equal(t, []string{"platform"}, b.startedWith())
	_, _, has := platform.Platform(catalog.KindTelegramBot)
	assert.True(t, has)

	offline := &bot{}
	offlineDir := directory{list: []connectors.Connector{offline}}
	log := &logged{}
	require.NoError(t, automations.StartPlatformConnectors(ctx, offlineDir, connectors.NewPlatformCredentials(offlineDir, creds), nil, fmt.Errorf("no database"), log.warn))
	assert.Empty(t, offline.startedWith(), "the reservation cannot be made: no webhook that could replace a tenant's")
	assert.NotEmpty(t, log.logs)
}

// tenantTakes tells whether a tenant's connection could take the account now: nil when it is free.
func tenantTakes(t *testing.T, db *pgxpool.Pool, tenant uuid.UUID, account string) error {
	t.Helper()
	ctx := context.Background()
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": bytes.Repeat([]byte{7}, 32)})
	if err != nil {
		return err
	}
	store := connectors.NewStore(db, keys, nil, nil, nil)
	name := "taker_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	conn, err := store.Save(ctx, connectors.Connection{TenantID: tenant, Kind: catalog.KindTelegramBot, Name: name, DisplayName: "Taker"}, connectors.Credentials{"token": account})
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		return connectors.ReserveIdentity(ctx, tx, catalog.KindTelegramBot, account, conn.ID)
	})
}

func expireLease(t *testing.T, db *pgxpool.Pool, account string) {
	t.Helper()
	_, err := db.Exec(context.Background(), `UPDATE automation.exclusive_identities SET lease_until = now() - interval '1 second' WHERE platform AND external = $1`, account)
	require.NoError(t, err)
}

// A rolling update that changes the platform bot: the new instances start with the new bot while the old ones still
// serve the previous one. The previous bot stays reserved while an old instance renews it — no tenant can connect it
// and set its webhook under them, and an old instance that starts again meanwhile does not release the new bot —
// and it is released once its lease ran out (the last old instance stopped).
func TestARollingUpdateKeepsThePreviousPlatformBotReservedWhileItRuns(t *testing.T) {
	db, tenant, _, _, _ := startEnv(t) //nolint:dogsled // the instances below bring their own bots
	ctx := context.Background()
	instance := func(token string) (*bot, connectors.Directory, connectors.PlatformCredentials) {
		b := &bot{}
		dir := directory{list: []connectors.Connector{b}}
		return b, dir, connectors.NewPlatformCredentials(dir, map[catalog.ConnectorKind]connectors.PlatformCredential{
			catalog.KindTelegramBot: {Credentials: connectors.Credentials{"token": token}}})
	}
	log := &logged{}
	oldBot, oldDir, oldPlatform := instance("old")
	require.NoError(t, automations.StartPlatformConnectors(ctx, oldDir, oldPlatform, db, nil, log.warn))
	assert.Equal(t, []string{"old"}, oldBot.startedWith())

	newBot, newDir, newPlatform := instance("new")
	require.NoError(t, automations.StartPlatformConnectors(ctx, newDir, newPlatform, db, nil, log.warn))
	assert.Equal(t, []string{"new"}, newBot.startedWith())
	require.ErrorIs(t, tenantTakes(t, db, tenant, "acct-old"), connectors.ErrIdentityTaken, "an old instance still serves the previous bot")
	require.ErrorIs(t, tenantTakes(t, db, tenant, "acct-new"), connectors.ErrIdentityTaken)

	// An old instance that starts again during the update keeps both reservations.
	againBot, againDir, againPlatform := instance("old")
	require.NoError(t, automations.StartPlatformConnectors(ctx, againDir, againPlatform, db, nil, log.warn))
	assert.Equal(t, []string{"old"}, againBot.startedWith())
	require.ErrorIs(t, tenantTakes(t, db, tenant, "acct-new"), connectors.ErrIdentityTaken, "the new bot is not released by an old start")

	// The old instances renew while they run; the renewal of the new ones does not release a live lease.
	automations.RenewPlatformConnectors(ctx, oldDir, oldPlatform, db, log.warn)
	automations.RenewPlatformConnectors(ctx, newDir, newPlatform, db, log.warn)
	require.ErrorIs(t, tenantTakes(t, db, tenant, "acct-old"), connectors.ErrIdentityTaken)

	// The last old instance stopped: its lease runs out, and the next renewal of a new instance releases it.
	expireLease(t, db, "acct-old")
	automations.RenewPlatformConnectors(ctx, newDir, newPlatform, db, log.warn)
	var left int
	require.NoError(t, db.QueryRow(ctx, `SELECT count(*) FROM automation.exclusive_identities WHERE platform AND external = 'acct-old'`).Scan(&left))
	assert.Zero(t, left, "the previous bot is released")
	require.NoError(t, tenantTakes(t, db, tenant, "acct-old"), "and a tenant may connect it")
	require.ErrorIs(t, tenantTakes(t, db, tenant, "acct-new"), connectors.ErrIdentityTaken, "the new bot stays the platform's")
	_, _, has := newPlatform.Platform(catalog.KindTelegramBot)
	assert.True(t, has)
}

// An expired lease is taken by a tenant's save directly, and an instance whose account a tenant took meanwhile
// withdraws the credential at its next renewal.
func TestAnExpiredPlatformLeaseIsFreeAndItsInstanceWithdraws(t *testing.T) {
	db, tenant, _, dir, creds := startEnv(t)
	ctx := context.Background()
	platform := connectors.NewPlatformCredentials(dir, creds)
	log := &logged{}
	require.NoError(t, automations.StartPlatformConnectors(ctx, dir, platform, db, nil, log.warn))
	expireLease(t, db, "acct-platform")
	require.NoError(t, tenantTakes(t, db, tenant, "acct-platform"), "a lease nobody renewed holds nothing")
	automations.RenewPlatformConnectors(ctx, dir, platform, db, log.warn)
	_, _, has := platform.Platform(catalog.KindTelegramBot)
	assert.False(t, has, "the account is a tenant's now: the credential is withdrawn")
}

// gatedBot holds its platform start until released, and counts the starts that run at once.
type gatedBot struct {
	bot
	entered chan struct{}
	release chan struct{}
	mu      sync.Mutex
	running int
	most    int
}

func (g *gatedBot) StartPlatform(ctx context.Context, c connectors.Credentials) error {
	g.mu.Lock()
	g.running++
	if g.running > g.most {
		g.most = g.running
	}
	g.mu.Unlock()
	g.entered <- struct{}{}
	<-g.release
	g.mu.Lock()
	g.running--
	g.mu.Unlock()
	return g.bot.StartPlatform(ctx, c)
}

// Two instances that start with the same platform bot at once (a rolling update) set its webhook one after the
// other: the start runs under the account's lock, which a tenant's save of the account takes too.
func TestPlatformStartsOfOneBotDoNotOverlap(t *testing.T) {
	db, tenant, _, _, creds := startEnv(t)
	ctx := context.Background()
	g := &gatedBot{entered: make(chan struct{}, 2), release: make(chan struct{})}
	dir := directory{list: []connectors.Connector{g}}
	errs := make(chan error, 3)
	for range 2 {
		go func() {
			errs <- automations.StartPlatformConnectors(ctx, dir, connectors.NewPlatformCredentials(dir, creds), db, nil, (&logged{}).warn)
		}()
	}
	<-g.entered // the first start sets the webhook up
	go func() { errs <- tenantTakes(t, db, tenant, "acct-platform") }()
	select {
	case <-g.entered:
		t.Fatal("a second start set the webhook while the first one did")
	case err := <-errs:
		t.Fatalf("a call finished while the first start held the account: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	g.release <- struct{}{}
	<-g.entered
	g.release <- struct{}{}
	var taken error
	for range 3 {
		err := <-errs
		if stderrors.Is(err, connectors.ErrIdentityTaken) {
			taken = err
			continue
		}
		require.NoError(t, err)
	}
	require.ErrorIs(t, taken, connectors.ErrIdentityTaken, "the tenant's save waited and found the account the platform's")
	assert.Equal(t, 1, g.most, "the starts never overlapped")
	assert.Equal(t, []string{"platform", "platform"}, g.startedWith())
}

// slowBot takes its time to set its webhook up.
type slowBot struct{ bot }

func (s *slowBot) StartPlatform(ctx context.Context, c connectors.Credentials) error {
	time.Sleep(3 * time.Second)
	return s.bot.StartPlatform(ctx, c)
}

// The lease counts from the moment the provider's setup returned, not from the start of the reservation's
// transaction: a slow setWebhook does not shorten it.
func TestThePlatformLeaseCountsFromTheEndOfTheStart(t *testing.T) {
	db, _, _, _, creds := startEnv(t) //nolint:dogsled // the test brings its own slow bot
	ctx := context.Background()
	slow := &slowBot{}
	dir := directory{list: []connectors.Connector{slow}}
	require.NoError(t, automations.StartPlatformConnectors(ctx, dir, connectors.NewPlatformCredentials(dir, creds), db, nil, (&logged{}).warn))
	var left float64
	require.NoError(t, db.QueryRow(ctx, `SELECT extract(epoch FROM lease_until - clock_timestamp()) FROM automation.exclusive_identities WHERE platform AND external = 'acct-platform'`).Scan(&left))
	assert.Greater(t, left, connectors.PlatformLease.Seconds()-2, "the full lease is left after a 3 s start")
}

// probeBot looks, while it sets its webhook up, at what another database session sees and at its own deadline.
type probeBot struct {
	bot
	db       *pgxpool.Pool
	seen     int
	deadline time.Duration
}

func (p *probeBot) StartPlatform(ctx context.Context, c connectors.Credentials) error {
	if d, ok := ctx.Deadline(); ok {
		p.deadline = time.Until(d)
	}
	if err := p.db.QueryRow(ctx, `SELECT count(*) FROM automation.exclusive_identities WHERE platform AND external = 'acct-platform'`).Scan(&p.seen); err != nil {
		return err
	}
	return p.bot.StartPlatform(ctx, c)
}

// The reservation is committed before the provider is set up: if the session that runs the setup ends during the
// call, the account stays reserved for the lease, and the setup is bounded well inside it, so a late setWebhook
// cannot replace the webhook of a tenant that took the account after it.
func TestThePlatformReservationIsCommittedBeforeTheWebhookIsSet(t *testing.T) {
	db, _, _, _, creds := startEnv(t) //nolint:dogsled // the test brings its own bot
	ctx := context.Background()
	p := &probeBot{db: db}
	dir := directory{list: []connectors.Connector{p}}
	require.NoError(t, automations.StartPlatformConnectors(ctx, dir, connectors.NewPlatformCredentials(dir, creds), db, nil, (&logged{}).warn))
	assert.Equal(t, 1, p.seen, "another session sees the reservation while the webhook is set")
	assert.Positive(t, p.deadline)
	assert.LessOrEqual(t, p.deadline, connectors.PlatformStartTimeout)
	assert.Less(t, connectors.PlatformStartTimeout, connectors.PlatformLease)
}

// The first rollout of this release runs next to instances of the previous one, which do not renew leases and, at their
// start, delete the platform's reservations of every other account (all of them when they run without a platform
// credential). Their statements, verbatim, cannot release a reservation whose lease is live; an expired one they may.
// An operator's release of all of them still works, and every released account is recorded as retired.
func TestThePreviousReleaseCannotReleaseALiveLease(t *testing.T) {
	db, _, _, dir, _ := startEnv(t) //nolint:dogsled // the instances are simulated by SQL
	ctx := context.Background()
	newer := connectors.NewPlatformCredentials(dir, map[catalog.ConnectorKind]connectors.PlatformCredential{
		catalog.KindTelegramBot: {Credentials: connectors.Credentials{"token": "new"}}})
	require.NoError(t, automations.StartPlatformConnectors(ctx, dir, newer, db, nil, (&logged{}).warn))
	held := func(account string) int {
		var n int
		require.NoError(t, db.QueryRow(ctx, `SELECT count(*) FROM automation.exclusive_identities WHERE platform AND external = $1`, account).Scan(&n))
		return n
	}
	require.Equal(t, 1, held("acct-new"))

	// The previous release's ReservePlatformIdentity for its own (old) bot, then its release without a credential.
	previousStart := func(account string) {
		require.NoError(t, pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `DELETE FROM automation.exclusive_identities WHERE platform AND kind = $1 AND external <> $2`, string(catalog.KindTelegramBot), account); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO automation.exclusive_identities (kind, external, platform) VALUES ($1, $2, TRUE) ON CONFLICT DO NOTHING`, string(catalog.KindTelegramBot), account)
			return err
		}))
	}
	previousStart("acct-old")
	assert.Equal(t, 1, held("acct-new"), "an instance of the previous release does not release the new bot")
	assert.Equal(t, 1, held("acct-old"), "and holds its own, for ten minutes without renewal")
	_, err := db.Exec(ctx, `DELETE FROM automation.exclusive_identities WHERE platform AND kind = $1`, string(catalog.KindTelegramBot))
	require.NoError(t, err)
	assert.Equal(t, 1, held("acct-new"), "nor does its release without a credential")

	expireLease(t, db, "acct-old")
	previousStart("acct-new")
	assert.Zero(t, held("acct-old"), "an expired lease is released by anyone")
	var retired int
	require.NoError(t, db.QueryRow(ctx, `SELECT count(*) FROM automation.retired_platform_identities WHERE external = 'acct-old'`).Scan(&retired))
	assert.Equal(t, 1, retired, "and recorded as retired")

	require.NoError(t, connectors.ReleasePlatformIdentities(ctx, db, catalog.KindTelegramBot))
	assert.Zero(t, held("acct-new"), "an operator's release releases a live lease too")
}

// A retired platform account is never forgotten: a tenant may connect it any time after, held or not by then.
func TestARetiredPlatformAccountIsNeverForgotten(t *testing.T) {
	db, tenant, _, _, _ := startEnv(t) //nolint:dogsled // no instance runs here
	ctx := context.Background()
	for _, account := range []string{"acct-platform", "acct-free"} {
		_, err := db.Exec(ctx, `INSERT INTO automation.retired_platform_identities (kind, external, retired_at) VALUES ($1, $2, now() - interval '30 days')`,
			string(catalog.KindTelegramBot), account)
		require.NoError(t, err)
	}
	require.NoError(t, connectors.PrunePlatformIdentities(ctx, db, catalog.KindTelegramBot))
	var left int
	require.NoError(t, db.QueryRow(ctx, `SELECT count(*) FROM automation.retired_platform_identities`).Scan(&left))
	assert.Equal(t, 2, left, "neither the unheld nor the held one is pruned")
	held := holdAccount(t, db, tenant) // a tenant takes acct-platform a month after it retired
	holders, err := connectors.RetiredPlatformHolders(ctx, db, catalog.KindTelegramBot)
	require.NoError(t, err)
	require.Len(t, holders, 1)
	assert.Equal(t, held, holders[0].ConnectionID)
}
