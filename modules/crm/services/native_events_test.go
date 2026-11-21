package services_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	crmpersistence "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	crmpermissions "github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/modules/logging"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// recordingProvider is a chat transport that sends nothing.
type recordingProvider struct{ sent []chat.Message }

func (p *recordingProvider) Transport() chat.Transport { return chat.SMSTransport }
func (p *recordingProvider) Send(_ context.Context, m chat.Message) error {
	p.sent = append(p.sent, m)
	return nil
}
func (p *recordingProvider) OnReceived(func(chat.Message) error) {}

type crmEventEnv struct {
	root     *testing.T // owns the environment; subtests commit through it
	env      *itf.TestEnvironment
	actor    user.User
	clients  *services.ClientService
	chats    *services.ChatService
	provider *recordingProvider
}

func newCRMEventEnv(t *testing.T) *crmEventEnv {
	t.Helper()
	env := itf.Setup(t, itf.WithComponents(
		logging.NewComponent(),
		core.NewComponent(&core.ModuleOptions{PermissionSchema: defaults.PermissionSchema()}),
		crm.NewComponent(),
	), itf.WithUser(itf.User(crmpermissions.ClientCreate, crmpermissions.ClientUpdate, crmpermissions.ClientRead, crmpermissions.ClientDelete)))
	actor, err := corepersistence.NewUserRepository(corepersistence.NewUploadRepository()).Create(env.Ctx,
		user.New("Operator", "Person", internet.MustParseEmail("operator-"+uuid.NewString()+"@example.com"), user.UILanguageEN, user.WithTenantID(env.TenantID())))
	require.NoError(t, err)
	clients := itf.GetService[services.ClientService](env)
	clientRepo, err := composition.Resolve[client.Repository](env.Container)
	require.NoError(t, err)
	provider := &recordingProvider{}
	chats := services.NewChatService(crmpersistence.NewChatRepository(), clientRepo, clients, []chat.Provider{provider}, eventbus.NewEventPublisher(logrus.New()))
	return &crmEventEnv{root: t, env: env, actor: actor, clients: clients, chats: chats, provider: provider}
}

// ctx acts as the operator in the test's current scope transaction (itf commits
// and renews it, so it is rebuilt on every use).
func (e *crmEventEnv) ctx() context.Context {
	ctx := composables.WithUser(e.env.Ctx, e.actor)
	return context.WithValue(ctx, constants.LoggerKey, logrus.NewEntry(logrus.New()))
}

func (e *crmEventEnv) newClient(t *testing.T, number string, opts ...client.Option) client.Client {
	t.Helper()
	p, err := phone.NewFromE164(number)
	require.NoError(t, err)
	c, err := client.New("Aziz", append([]client.Option{client.WithTenantID(e.env.TenantID()), client.WithPhone(p)}, opts...)...)
	require.NoError(t, err)
	require.NoError(t, e.clients.Create(e.env.Ctx, c))
	created, err := e.clients.GetByPhone(e.env.Ctx, p.Value())
	require.NoError(t, err)
	return created
}

// commitsAndRollsBack is itf.TestEnvironment.OutboxCommitsAndRollsBack acting
// as the operator, who must be the initiator of every event.
func (e *crmEventEnv) commitsAndRollsBack(t *testing.T, ids []string, want map[string]int, domain itf.Domain, change func(ctx context.Context, suffix string) error) map[string][]itf.OutboxRow {
	t.Helper()
	e.env.CommitTx(e.root)
	got := e.env.OutboxCommitsAndRollsBack(t, e.ctx(), e.env.TenantID(), ids, want, domain, change)
	for id, rows := range got {
		for _, r := range rows {
			require.NotNil(t, r.Initiator, "%s: the acting user is the event initiator", id)
			require.EqualValues(t, e.actor.ID(), *r.Initiator)
		}
	}
	return got
}

// scopeCounts is the number of outbox rows of each event in the scope transaction.
func (e *crmEventEnv) scopeCounts(t *testing.T, ids ...string) map[string]int {
	t.Helper()
	out := make(map[string]int, len(ids))
	for _, id := range ids {
		out[id] = len(itf.OutboxRowsIn(t, e.ctx(), e.env.TenantID(), id))
	}
	return out
}

func TestClientServiceEvents_CommitAndRollBackWithTheClient(t *testing.T) {
	e := newCRMEventEnv(t)
	ids := []string{services.ClientCreatedEventID, services.ClientUpdatedEventID, services.ClientDeletedEventID}

	t.Run("created", func(t *testing.T) {
		got := e.commitsAndRollsBack(t, ids, map[string]int{services.ClientCreatedEventID: 1}, itf.Domain{SQL: "SELECT count(*)::text FROM clients WHERE tenant_id = $1", Args: []any{e.env.TenantID()}}, func(ctx context.Context, suffix string) error {
			number := "+99890333" + map[string]string{"rolled-back": "0001", "committed": "0002"}[suffix]
			p, err := phone.NewFromE164(number)
			if err != nil {
				return err
			}
			c, err := client.New("Created", client.WithTenantID(e.env.TenantID()), client.WithPhone(p), client.WithLastName("Client"))
			if err != nil {
				return err
			}
			return e.clients.Create(ctx, c)
		})
		row := got[services.ClientCreatedEventID][0]
		require.Equal(t, "crm.client", row.SubjectEntity)
		var p services.ClientCreatedPayload
		require.NoError(t, json.Unmarshal(row.Payload, &p))
		require.Equal(t, "Created", p.FirstName)
		require.Equal(t, "Client", *p.LastName)
		require.Equal(t, fmt.Sprint(p.ClientID), row.SubjectKey)
		require.Equal(t, "998903330002", *p.Phone)
	})

	t.Run("updated", func(t *testing.T) {
		c := e.newClient(t, "+998903330011")
		got := e.commitsAndRollsBack(t, ids, map[string]int{services.ClientUpdatedEventID: 1}, itf.Domain{SQL: "SELECT first_name FROM clients WHERE id = $1", Args: []any{c.ID()}}, func(ctx context.Context, suffix string) error {
			return e.clients.Update(ctx, c.SetName("Renamed-"+suffix, "", "").SetComments("VIP"))
		})
		var p services.ClientUpdatedPayload
		require.NoError(t, json.Unmarshal(got[services.ClientUpdatedEventID][0].Payload, &p))
		require.Equal(t, "Renamed-committed", p.FirstName)
		require.Equal(t, []string{"comments", "firstName"}, p.Changed)

		// An update that changes nothing publishes nothing.
		before := e.scopeCounts(t, ids...)
		require.NoError(t, e.clients.Update(e.ctx(), c.SetName("Renamed-committed", "", "").SetComments("VIP")))
		require.Equal(t, before, e.scopeCounts(t, ids...))
	})

	t.Run("deleted", func(t *testing.T) {
		c := e.newClient(t, "+998903330021")
		got := e.commitsAndRollsBack(t, ids, map[string]int{services.ClientDeletedEventID: 1}, itf.Domain{SQL: "SELECT count(*)::text FROM clients WHERE id = $1", Args: []any{c.ID()}}, func(ctx context.Context, _ string) error {
			_, err := e.clients.Delete(ctx, c.ID())
			return err
		})
		require.JSONEq(t, fmt.Sprintf(`{"clientId":"%d","firstName":"Aziz"}`, c.ID()), string(got[services.ClientDeletedEventID][0].Payload))
	})

	t.Run("created from an incoming chat number", func(t *testing.T) {
		got := e.commitsAndRollsBack(t, ids, map[string]int{services.ClientCreatedEventID: 1}, itf.Domain{SQL: "SELECT count(*)::text FROM clients WHERE tenant_id = $1", Args: []any{e.env.TenantID()}}, func(ctx context.Context, suffix string) error {
			_, err := e.chats.CreateOrGetClientByPhone(ctx, "+99890333"+map[string]string{"rolled-back": "0031", "committed": "0032"}[suffix])
			return err
		})
		var p services.ClientCreatedPayload
		require.NoError(t, json.Unmarshal(got[services.ClientCreatedEventID][0].Payload, &p))
		require.Equal(t, "Guest", p.FirstName)

		// The existing client of that number is no new client.
		before := e.scopeCounts(t, ids...)
		_, err := e.chats.CreateOrGetClientByPhone(e.ctx(), "+998903330032")
		require.NoError(t, err)
		require.Equal(t, before, e.scopeCounts(t, ids...))
	})
}

func TestChatEvents_MessageAddedCommitsAndRollsBackWithTheMessage(t *testing.T) {
	e := newCRMEventEnv(t)
	ids := []string{services.ChatMessageAddedEventID}
	c := e.newClient(t, "+998903330041")
	chatEntity, err := e.chats.GetByClientIDOrCreate(e.ctx(), c.ID())
	require.NoError(t, err)

	t.Run("inbound", func(t *testing.T) {
		long := strings.Repeat("ж", services.ChatMessageBodyLimit+500)
		got := e.commitsAndRollsBack(t, ids, map[string]int{services.ChatMessageAddedEventID: 1}, itf.Domain{SQL: "SELECT count(*)::text FROM messages WHERE chat_id = $1", Args: []any{chatEntity.ID()}}, func(ctx context.Context, _ string) error {
			sender := chat.NewMember(chat.NewClientSender(c.ID(), 0, c.FirstName(), c.LastName()), chat.TelegramTransport, chat.WithMemberTenantID(e.env.TenantID()))
			_, err := e.chats.AddMessageToChat(ctx, chatEntity.ID(), chat.NewMessage(long, sender))
			return err
		})
		row := got[services.ChatMessageAddedEventID][0]
		require.Equal(t, "crm.chat", row.SubjectEntity)
		require.Equal(t, fmt.Sprint(chatEntity.ID()), row.SubjectKey)
		var p services.ChatMessageAddedPayload
		require.NoError(t, json.Unmarshal(row.Payload, &p))
		require.EqualValues(t, chatEntity.ID(), p.ChatID)
		require.EqualValues(t, c.ID(), p.ClientID)
		require.NotZero(t, p.MessageID)
		require.Equal(t, "telegram", p.Transport)
		require.Equal(t, "inbound", p.Direction)
		require.Equal(t, strings.Repeat("ж", services.ChatMessageBodyLimit), p.Body, "the text is cut to 4000 characters")
		require.False(t, p.SentAt.IsZero())
	})

	t.Run("outbound", func(t *testing.T) {
		got := e.commitsAndRollsBack(t, ids, map[string]int{services.ChatMessageAddedEventID: 1}, itf.Domain{SQL: "SELECT count(*)::text FROM messages WHERE chat_id = $1", Args: []any{chatEntity.ID()}}, func(ctx context.Context, suffix string) error {
			_, err := e.chats.SendMessage(ctx, services.SendMessageCommand{ChatID: chatEntity.ID(), Transport: chat.SMSTransport, Message: "Hello " + suffix})
			return err
		})
		var p services.ChatMessageAddedPayload
		require.NoError(t, json.Unmarshal(got[services.ChatMessageAddedEventID][0].Payload, &p))
		require.Equal(t, "outbound", p.Direction)
		require.Equal(t, "sms", p.Transport)
		require.Equal(t, "Hello committed", p.Body)
		require.NotEmpty(t, e.provider.sent)
	})

	t.Run("a message is published once", func(t *testing.T) {
		before := len(itf.OutboxRowsIn(t, e.ctx(), e.env.TenantID(), services.ChatMessageAddedEventID))
		_, err := e.chats.SendMessage(e.ctx(), services.SendMessageCommand{ChatID: chatEntity.ID(), Transport: chat.SMSTransport, Message: "Second"})
		require.NoError(t, err)
		_, err = e.chats.SendMessage(e.ctx(), services.SendMessageCommand{ChatID: chatEntity.ID(), Transport: chat.SMSTransport, Message: "Third"})
		require.NoError(t, err)
		rows := itf.OutboxRowsIn(t, e.ctx(), e.env.TenantID(), services.ChatMessageAddedEventID)
		require.Len(t, rows, before+2, "earlier messages of the chat are not published again")
	})
}

// A passport-only or a PIN-only edit still publishes crm.client.updated.v1,
// naming the field and never its value.
func TestClientServiceEvents_PassportAndPinEditsArePublished(t *testing.T) {
	e := newCRMEventEnv(t)
	ids := []string{services.ClientUpdatedEventID}
	c := e.newClient(t, "+998903330051")
	pin, err := tax.NewPin("12345678901234", country.Uzbekistan)
	require.NoError(t, err)

	for _, tc := range []struct {
		name   string
		field  string
		column string
		edit   func(client.Client) client.Client
	}{
		{"passport", "passport", "passport_id::text", func(c client.Client) client.Client { return c.SetPassport(passport.New("AB", "7654321")) }},
		{"pin", "pin", "pin", func(c client.Client) client.Client { return c.SetPIN(pin) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := e.commitsAndRollsBack(t, ids, map[string]int{services.ClientUpdatedEventID: 1},
				itf.Domain{SQL: "SELECT COALESCE(" + tc.column + ", 'none') FROM clients WHERE id = $1", Args: []any{c.ID()}},
				func(ctx context.Context, _ string) error { return e.clients.Update(ctx, tc.edit(c)) })
			row := got[services.ClientUpdatedEventID][0]
			var p services.ClientUpdatedPayload
			require.NoError(t, json.Unmarshal(row.Payload, &p))
			require.Equal(t, []string{tc.field}, p.Changed)
			for _, secret := range []string{"7654321", "12345678901234"} {
				require.NotContains(t, string(row.Payload), secret, "the value of %s never travels", tc.field)
			}
			// Re-read: the next edit starts from the stored client.
			c, err = e.clients.GetByID(e.env.Ctx, c.ID())
			require.NoError(t, err)
		})
	}
}

// Appends to one chat from concurrent transactions publish every message once:
// the chat is locked before it is read, so no transaction reports a message
// another one stored.
func TestChatEvents_ConcurrentAppendsPublishEachMessageOnce(t *testing.T) {
	e := newCRMEventEnv(t)
	c := e.newClient(t, "+998903330061")
	chatEntity, err := e.chats.GetByClientIDOrCreate(e.ctx(), c.ID())
	require.NoError(t, err)
	e.env.CommitTx(t)
	detached := context.WithValue(context.WithValue(e.ctx(), constants.TxKey, nil), constants.AfterCommitKey, nil)

	const appends = 8
	var wg sync.WaitGroup
	errs := make(chan error, appends)
	start := make(chan struct{})
	for i := 0; i < appends; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			sender := chat.NewMember(chat.NewClientSender(c.ID(), 0, c.FirstName(), c.LastName()), chat.TelegramTransport, chat.WithMemberTenantID(e.env.TenantID()))
			_, err := e.chats.AddMessageToChat(detached, chatEntity.ID(), chat.NewMessage(fmt.Sprintf("message %d", i), sender))
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	seen := map[int64]int{}
	for _, row := range e.env.OutboxRows(t, e.env.TenantID(), services.ChatMessageAddedEventID) {
		var p services.ChatMessageAddedPayload
		require.NoError(t, json.Unmarshal(row.Payload, &p))
		seen[p.MessageID]++
	}
	require.Len(t, seen, appends, "every appended message is published")
	for id, n := range seen {
		require.Equal(t, 1, n, "message %d is published once", id)
	}
}
