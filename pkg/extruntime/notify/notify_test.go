package notify_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/notifications/notificationstest"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage/managetest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/notify"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool/pooltest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

const (
	ns         = "acme.insurance"
	noticeID   = ns + ".renewalReminder.v1"
	resolverID = ns + ".customerContacts"
	customerID = ns + ".customer"
	eventKey   = "acme.insurance.renewal_reminder.v1"
)

type noGuard struct{}

func (noGuard) CheckWrite(context.Context, pgx.Tx, spi.Scope, spi.WriteChange) error { return nil }

type subjects struct{}

func (s subjects) ResolveSubject(_ context.Context, tenant uuid.UUID, userID int64) (spi.Subject, error) {
	return spi.Subject{TenantID: tenant, UserID: userID, Kind: spi.SubjectOwner, Permissions: []string{"Customer.Read"}}, nil
}

type nenv struct {
	t         *testing.T
	f         *itf.TestEnvironment
	committed context.Context
	sms       *notificationstest.Adapter
	email     *notificationstest.Adapter
	dispatch  *services.NotificationDispatchService
	channels  *services.NotificationChannelService
	notices   *notifications.Catalog
	renderers *notifications.ContentRenderers
	contacts  *notifications.ContactResolvers
	svc       *manage.Service
	cat       *catalog.Catalog
	host      *host.Host
	adapter   *notify.Adapter
	notifier  *notify.Notifier
	admin     spi.Subject
	owner     int64
}

func newNenv(t *testing.T) *nenv {
	t.Helper()
	e := &nenv{t: t, sms: notificationstest.NewSMS(), email: notificationstest.NewEmail()}
	push := notificationstest.NewPush()
	components := modules.Components()
	for i, c := range components {
		if c.Descriptor().Name == "core" {
			components[i] = core.NewComponent(&core.ModuleOptions{
				DisableNotificationWorker: true, PermissionSchema: defaults.PermissionSchema(),
				NotificationChannels: []notifications.ChannelAdapter{e.email, e.sms, push},
			})
		}
	}
	e.f = itf.Setup(t, itf.WithComponents(components...), itf.WithUser(itf.User()))
	e.dispatch = itf.GetService[services.NotificationDispatchService](e.f)
	e.channels = itf.GetService[services.NotificationChannelService](e.f)
	e.notices = itf.GetService[notifications.Catalog](e.f)
	e.renderers = itf.GetService[notifications.ContentRenderers](e.f)
	e.contacts = itf.GetService[notifications.ContactResolvers](e.f)
	require.NoError(t, e.f.Tx.Commit(e.f.Ctx))
	ctx := context.Background()
	ctx = composables.WithPool(ctx, e.f.Pool)
	ctx = composables.WithTenantID(ctx, e.f.TenantID())
	ctx = composables.WithParams(ctx, itf.DefaultParams())
	ctx = composables.WithSession(ctx, itf.MockSession())
	ctx = composables.WithUser(ctx, e.f.User)
	e.committed = itf.TestExecution(ctx)

	e.owner = e.newUser("owner")
	e.admin = spi.Subject{TenantID: e.f.TenantID(), UserID: e.owner, Kind: spi.SubjectInitiator,
		Permissions: []string{manage.PermRead, manage.PermAuthor, manage.PermReview, manage.PermPublish}}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	arts, err := artifact.NewStore(e.f.Pool, t.TempDir())
	require.NoError(t, err)
	natives := catalog.NewNativeSet(data.DefaultNatives())
	e.cat = catalog.New(e.f.Pool, natives)
	e.svc, err = manage.New(manage.Config{DB: e.f.Pool, Builder: &managetest.Builder{}, Kits: &managetest.Kits{}, Artifacts: arts, Catalog: e.cat, Natives: data.DefaultNatives()})
	require.NoError(t, err)

	p, err := pool.New(pool.Config{Size: 1, WorkerScript: pooltest.BuildWorker(t), CancelGrace: 300 * time.Millisecond, RestartBackoffMin: 20 * time.Millisecond, Logger: logger})
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Close(context.Background()) })
	e.host, err = host.New(host.Config{
		Pool: p, DB: e.f.Pool, Bindings: host.NewPgBindings(e.f.Pool, e.svc), Catalog: e.cat,
		Data: data.NewStore(e.cat, data.DefaultNatives(), noGuard{}), Logger: logger,
	})
	require.NoError(t, err)

	e.adapter, err = notify.NewAdapter(notify.Config{
		Catalog: e.cat, Notices: e.notices, Renderers: e.renderers, Contacts: e.contacts, Runner: e.host,
		Artifacts: e.svc, Manifests: e.svc, Owners: e.svc, Subjects: subjects{}, Published: notify.PgPublished{DB: e.f.Pool}, Logger: logger,
	})
	require.NoError(t, err)
	e.notifier = notify.NewNotifier(e.dispatch, e.cat, e.svc, e.adapter)
	return e
}

func (e *nenv) newUser(name string) int64 {
	e.t.Helper()
	var id int64
	require.NoError(e.t, e.f.Pool.QueryRow(context.Background(),
		`INSERT INTO users (tenant_id, type, first_name, last_name, email, ui_language)
		 VALUES ($1, 'user', $2, 'Tester', $3, 'en') RETURNING id`,
		e.f.TenantID(), name, name+"-"+uuid.NewString()+"@example.com").Scan(&id))
	return id
}

func (e *nenv) count(sql string, args ...any) int {
	e.t.Helper()
	var n int
	require.NoError(e.t, e.f.Pool.QueryRow(e.committed, sql, args...).Scan(&n))
	return n
}

func (e *nenv) drain() {
	e.t.Helper()
	for range 10 {
		worked, err := e.dispatch.Process(e.committed)
		require.NoError(e.t, err)
		if !worked {
			break
		}
	}
	_, err := e.channels.Process(e.committed)
	require.NoError(e.t, err)
}

func str() manifest.Type { return manifest.Type{Kind: manifest.KindString} }

func payloadType() *manifest.Type {
	return &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
		{Name: "number", Type: str()}, {Name: "expiresOn", Type: manifest.Type{Kind: manifest.KindDate}},
	}}
}

func serverSource(tag string) string {
	return fmt.Sprintf(`export const reminderTemplates = { renderers: {
  sms: ({ payload, subject, locale }) => "%[1]s Policy " + payload.number + " (" + subject.key + ") expires " + payload.expiresOn + " [" + locale + "]",
  email: ({ payload }) => ({ subject: "%[1]s Renewal " + payload.number, html: "<p>" + payload.number + "</p>", text: "Renewal " + payload.number }),
  inApp: ({ payload }) => ({ title: "Renewal", body: payload.number }),
} };
export const customerContacts = async (q, record) => ({ sms: ["+99890" + record.key], email: [record.key + "@example.com"] });
`, tag)
}

func (e *nenv) publish(version string) {
	e.t.Helper()
	ctx := context.Background()
	m := manifest.Manifest{
		Schema: manifest.SchemaVersion, Namespace: ns,
		Resources: []manifest.Resource{
			{ID: customerID, Kind: manifest.ResEntity, Fields: []manifest.Field{{ID: "name", Type: str()}}},
			{ID: noticeID, Kind: manifest.ResNotice, Label: manifest.Label{"en": "Renewal reminder", "ru": "Напоминание"}, Payload: payloadType(), Subject: customerID},
			{ID: resolverID, Kind: manifest.ResContactResolver, Subject: customerID},
		},
		Implementations: []manifest.Implementation{{Export: "customerContacts", Implements: resolverID, Kind: manifest.ImplContactResolver}},
		Templates:       []manifest.Template{{Export: "reminderTemplates", Notice: noticeID, Channels: []string{"sms", "email", "inApp"}}},
	}
	mj, err := json.Marshal(m)
	require.NoError(e.t, err)
	if _, err := e.svc.GetPackage(ctx, e.admin, ns); err != nil {
		_, err = e.svc.CreatePackage(ctx, e.admin, ns, "")
		require.NoError(e.t, err)
	}
	pj := fmt.Sprintf(`{"name":"@acme/ins","version":%q,"iota":{"namespace":%q}}`, version, ns)
	for p, c := range map[string]string{"package.json": pj, "manifest.json": string(mj), "server.mjs": serverSource("V" + version), "contracts.mjs": "export {}\n"} {
		_, err := e.svc.PutFile(ctx, e.admin, ns, p, []byte(c), nil)
		require.NoError(e.t, err)
	}
	_, err = e.svc.Publish(ctx, e.admin, ns, version)
	require.NoError(e.t, err)
	_, err = e.svc.Review(ctx, e.admin, ns, version, true, "ok")
	require.NoError(e.t, err)
}

func (e *nenv) activate(version string) {
	e.t.Helper()
	_, err := e.svc.Activate(context.Background(), e.admin, ns, version)
	require.NoError(e.t, err)
	require.NoError(e.t, e.adapter.Sync(context.Background()))
}

func (e *nenv) scopeOf(c spi.Caller) spi.Scope {
	return spi.Scope{Subject: spi.Subject{TenantID: e.f.TenantID(), UserID: e.owner, Kind: spi.SubjectInitiator}, Caller: c}
}

func (e *nenv) intent(number string, recipients ...spi.Recipient) spi.NoticeIntent {
	payload, err := json.Marshal(map[string]string{"number": number, "expiresOn": "2026-12-01"})
	require.NoError(e.t, err)
	return spi.NoticeIntent{
		Notice: noticeID, Version: "", Subject: &spi.ObjectRef{Entity: customerID, Key: "p-1"}, Payload: payload,
		Recipients: recipients, Channels: []string{"sms", "email"},
		Producer: spi.Producer{InvocationID: uuid.NewString()},
	}
}

func address() spi.Recipient {
	return spi.Recipient{Kind: spi.RecipientAddress, Address: map[string]string{"sms": "+998900000001", "email": "holder@example.com"}}
}

// send enqueues in a business transaction that commits (or rolls back).
func (e *nenv) send(version string, commit bool, in spi.NoticeIntent) (string, error) {
	return e.sendAs(spi.Caller{Package: ns, Version: version}, commit, in)
}

func (e *nenv) sendAs(c spi.Caller, commit bool, in spi.NoticeIntent) (string, error) {
	e.t.Helper()
	tx, err := e.f.Pool.Begin(e.committed)
	require.NoError(e.t, err)
	id, err := e.notifier.Enqueue(e.committed, tx, e.scopeOf(c), in)
	if err != nil || !commit {
		require.NoError(e.t, tx.Rollback(e.committed))
		return id, err
	}
	require.NoError(e.t, tx.Commit(e.committed))
	return id, nil
}

func TestNoticeIsEnqueuedAtomicallyAndRenderedByTheTemplateInNode(t *testing.T) {
	e := newNenv(t)
	e.publish("1.0.0")
	e.activate("1.0.0")

	// the package's notice is published in the notification catalog with its snapshot
	def, ok := e.notices.Get(eventKey)
	require.True(t, ok)
	require.Equal(t, "extruntime:"+ns, def.Source)
	require.Equal(t, ns+"@1.0.0", def.SnapshotRef)
	require.Equal(t, "Renewal reminder", def.Name["en"])

	// a rolled back business transaction leaves nothing behind
	_, err := e.send("1.0.0", false, e.intent("P-1", address()))
	require.NoError(t, err)
	require.Zero(t, e.count("SELECT COUNT(*) FROM core.notification_notices WHERE tenant_id=$1", e.f.TenantID()))
	require.Zero(t, e.count("SELECT COUNT(*) FROM core.notification_dispatches WHERE tenant_id=$1", e.f.TenantID()))
	e.drain()
	require.Empty(t, e.sms.Calls())

	// a payload that violates the notice contract is refused before commit
	bad := e.intent("P-1", address())
	bad.Payload = json.RawMessage(`{"number": 7}`)
	_, err = e.send("1.0.0", true, bad)
	require.Error(t, err)
	require.Zero(t, e.count("SELECT COUNT(*) FROM core.notification_notices WHERE tenant_id=$1", e.f.TenantID()))
	noRecipientChannel := e.intent("P-1", spi.Recipient{Kind: spi.RecipientAddress, Address: map[string]string{"fax": "1"}})
	_, err = e.send("1.0.0", true, noRecipientChannel)
	require.Error(t, err)

	// committed: one notice pinned to the package version
	id, err := e.send("1.0.0", true, e.intent("P-1", address()))
	require.NoError(t, err)
	require.Equal(t, 1, e.count("SELECT COUNT(*) FROM core.notification_notices WHERE tenant_id=$1 AND id=$2::uuid AND snapshot_ref=$3", e.f.TenantID(), id, ns+"@1.0.0"))
	require.Empty(t, e.sms.Calls(), "nothing is rendered or sent inside the producer's transaction")

	// delivery renders through the template export in a real Node worker
	e.drain()
	sms := e.sms.SentTo("+998900000001")
	require.Len(t, sms, 1)
	require.Equal(t, "V1.0.0 Policy P-1 (p-1) expires 2026-12-01 [en]", sms[0].Text)
	require.Equal(t, ns+"@1.0.0", sms[0].SnapshotRef)
	mail := e.email.SentTo("holder@example.com")
	require.Len(t, mail, 1)
	require.Equal(t, "V1.0.0 Renewal P-1", mail[0].EmailSubject)
	require.Equal(t, "<p>P-1</p>", mail[0].HTML)
	require.Equal(t, 2, e.count(`SELECT COUNT(*) FROM extruntime.invocations WHERE contract = $1 AND outcome = 'ok'`, noticeID), "each rendering is an invocation of the template")
}

func TestRecordRecipientUsesThePackagesContactResolver(t *testing.T) {
	e := newNenv(t)
	e.publish("1.0.0")
	e.activate("1.0.0")
	_, ok := e.contacts.Lookup(customerID)
	require.True(t, ok, "the package's resolver is registered for its entity")

	in := e.intent("P-2", spi.Recipient{Kind: spi.RecipientRecord, Record: &spi.ObjectRef{Entity: customerID, Key: "c-77"}})
	in.Channels = []string{"sms"}
	_, err := e.send("1.0.0", true, in)
	require.NoError(t, err)

	// the resolver runs as the tenant's trigger owner; without one the delivery waits
	e.drain()
	require.Empty(t, e.sms.Sent())
	_, err = e.svc.AssignOwner(context.Background(), e.admin, resolverID, e.owner)
	require.NoError(t, err)
	_, err = e.f.Pool.Exec(e.committed, "UPDATE core.notification_deliveries SET available_at=NOW() WHERE tenant_id=$1 AND status IN ('pending','unresolved')", e.f.TenantID())
	require.NoError(t, err)
	e.drain()
	sent := e.sms.SentTo("+99890c-77")
	require.Len(t, sent, 1, "record -> contact targets resolved by the package's server code")
	require.Contains(t, sent[0].Text, "Policy P-2")
}

func TestPendingDeliveriesKeepResolvingTheirSnapshotAfterDeactivationAndNewVersions(t *testing.T) {
	e := newNenv(t)
	ctx := context.Background()
	e.publish("1.0.0")
	e.activate("1.0.0")

	queued, err := e.send("1.0.0", true, e.intent("P-3", address()))
	require.NoError(t, err)
	require.NotEmpty(t, queued)

	// deactivation takes the notice out of the active catalog ...
	require.NoError(t, e.svc.Deactivate(ctx, e.admin, ns))
	require.NoError(t, e.adapter.Sync(ctx))
	_, ok := e.notices.Get(eventKey)
	require.False(t, ok)
	_, err = e.sendAs(spi.Caller{Package: "acme.other", Version: "1"}, true, e.intent("P-4", address()))
	require.Error(t, err, "once deactivated, nobody else gets its templates")
	require.Equal(t, 1, e.count("SELECT COUNT(*) FROM core.notification_notices WHERE tenant_id=$1", e.f.TenantID()))
	// an instance pinned to the deactivated version still sends with its own templates
	_, err = e.send("1.0.0", true, e.intent("P-4b", address()))
	require.NoError(t, err)
	require.Equal(t, 2, e.count("SELECT COUNT(*) FROM core.notification_notices WHERE tenant_id=$1", e.f.TenantID()))

	// the queued ones still render with their own version's template
	e.drain()
	v1sent := e.sms.SentTo("+998900000001")
	v1texts := make([]string, 0, len(v1sent))
	for _, m := range v1sent {
		v1texts = append(v1texts, m.Text)
	}
	require.ElementsMatch(t, []string{
		"V1.0.0 Policy P-3 (p-1) expires 2026-12-01 [en]",
		"V1.0.0 Policy P-4b (p-1) expires 2026-12-01 [en]",
	}, v1texts)

	// the same version can come back
	e.activate("1.0.0")
	_, err = e.send("1.0.0", true, e.intent("P-5", address()))
	require.NoError(t, err, "explicit sends resolve through the snapshot")

	// v2 changes the template; what v1 queued still renders as v1
	e.publish("2.0.0")
	e.activate("2.0.0")
	def, ok := e.notices.Get(eventKey)
	require.True(t, ok)
	require.Equal(t, ns+"@2.0.0", def.SnapshotRef)
	_, err = e.send("2.0.0", true, e.intent("P-6", spi.Recipient{Kind: spi.RecipientAddress, Address: map[string]string{"sms": "+998900000002"}}))
	require.NoError(t, err)
	e.drain()
	sent := e.sms.Sent()
	texts := make([]string, 0, len(sent))
	for _, m := range sent {
		texts = append(texts, m.Text)
	}
	require.Contains(t, texts, "V1.0.0 Policy P-5 (p-1) expires 2026-12-01 [en]", "queued under v1")
	require.Contains(t, texts, "V2.0.0 Policy P-6 (p-1) expires 2026-12-01 [en]", "queued under v2")

	// after a restart (fresh registries) old snapshots resolve again
	notices := notifications.NewCatalog()
	renderers := notifications.NewContentRenderers(notices)
	adapter, err := notify.NewAdapter(notify.Config{
		Catalog: e.cat, Notices: notices, Renderers: renderers, Runner: e.host, Artifacts: e.svc, Manifests: e.svc,
		Published: notify.PgPublished{DB: e.f.Pool},
	})
	require.NoError(t, err)
	require.NoError(t, adapter.Sync(ctx))
	event := notifications.Event{Key: eventKey, SnapshotRef: ns + "@1.0.0", TenantID: e.f.TenantID(), Payload: json.RawMessage(`{"number":"P-9","expiresOn":"2026-12-01"}`),
		Subject: notifications.SubjectReference{Type: customerID, ID: "p-1"}}
	_, ok = notices.For(event)
	require.True(t, ok)
	content, err := renderers.Render(ctx, notifications.RenderRequest{Event: event, Channel: notifications.ChannelSMS, Language: "en"})
	require.NoError(t, err)
	require.Equal(t, "V1.0.0 Policy P-9 (p-1) expires 2026-12-01 [en]", content.Text)
	_, active := notices.Get(eventKey)
	require.True(t, active)
	got, _ := notices.Get(eventKey)
	require.Equal(t, ns+"@2.0.0", got.SnapshotRef, "only the active version is published; v1 is a retained snapshot")

	// a channel the template does not provide is a permanent failure
	_, err = renderers.Render(ctx, notifications.RenderRequest{Event: event, Channel: notifications.ChannelPush, Language: "en"})
	require.ErrorIs(t, err, notifications.ErrPermanent)
}

func TestEventKeyMapsNoticeIDs(t *testing.T) {
	require.Equal(t, "acme.insurance.renewal_reminder.v1", notify.EventKey("acme.insurance.renewalReminder.v1"))
	require.Equal(t, "acme.insurance.policy_expired.v1", notify.EventKey("acme.insurance.policyExpired"))
	require.Equal(t, "acme.insurance.sms2_send.v3", notify.EventKey("acme.insurance.sms2Send.v3"))
	pkg, ver, ok := notify.SplitRef("acme.insurance@1.0.0")
	require.True(t, ok)
	require.Equal(t, []string{"acme.insurance", "1.0.0"}, []string{pkg, ver})
}
