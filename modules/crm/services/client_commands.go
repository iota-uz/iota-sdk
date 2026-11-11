package services

import (
	"context"
	"strconv"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	crmPerms "github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ClientEntity is the native entity ID of CRM clients (refs travel as
// {"entity":"crm.client","key":"<id>"}).
const ClientEntity = "crm.client"

// SaveClientInput creates a client (no Client ref) or patches one: a present
// field is applied, an absent one is left as it is. Strings that would build
// invalid value objects (phone, email, PIN) are validation issues at their
// path.
type SaveClientInput struct {
	Client         *spi.ObjectRef `json:"client,omitempty" wire:"ref=crm.client"`
	FirstName      *string        `json:"firstName,omitempty"`
	LastName       *string        `json:"lastName,omitempty"`
	MiddleName     *string        `json:"middleName,omitempty"`
	Phone          *string        `json:"phone,omitempty"`
	Email          *string        `json:"email,omitempty"`
	Address        *string        `json:"address,omitempty"`
	PassportSeries *string        `json:"passportSeries,omitempty"`
	PassportNumber *string        `json:"passportNumber,omitempty"`
	Pin            *string        `json:"pin,omitempty"`
	CountryCode    *string        `json:"countryCode,omitempty"`
	Comments       *string        `json:"comments,omitempty"`
}

// SaveClientOutput is the saved client.
type SaveClientOutput struct {
	Client  spi.ObjectRef `json:"client" wire:"ref=crm.client"`
	Created bool          `json:"created"`
}

// Issue codes of the client contracts.
const (
	IssueClientNotFound  = "crm.clientNotFound"
	IssueInvalidPhone    = "crm.invalidPhone"
	IssueInvalidEmail    = "crm.invalidEmail"
	IssueInvalidPin      = "crm.invalidPin"
	IssueInvalidCountry  = "crm.invalidCountry"
	IssueInvalidClientID = "crm.invalidClientRef"
)

// UpsertClient is the reusable invariant operation behind every client
// write: authorization (Client.Create / Client.Update), value-object
// validation, persistence through the repository with the change history in
// the same transaction, and the client events after commit. Packages reach
// CRM clients only through it (generic writes to crm.client are refused); a
// package guard on it applies to every caller.
var UpsertClient = native.MustRegister(native.Operation("crm.operations.upsertClient", native.Spec{
	Label:       map[string]string{"en": "Save client data", "ru": "Сохранить данные клиента", "uz": "Mijoz ma'lumotlarini saqlash"},
	Description: "Creates a CRM client, or applies the present fields to an existing one, enforcing the client invariants.",
}, func(d native.Deps) (native.OperationFunc[SaveClientInput, SaveClientOutput], error) {
	svc, err := native.Use[*ClientService](d)
	if err != nil {
		return nil, err
	}
	return svc.upsert, nil
}))

// SaveClient is the published intent of the client forms: an ordinary
// command whose Go default runs UpsertClient in one transaction. A package
// may replace it (a command binding) with its own algorithm; the replacement
// still writes through UpsertClient, so its invariants and guards hold.
var SaveClient = native.MustRegister(native.Command("crm.commands.saveClient", native.Spec{
	Label:       map[string]string{"en": "Save client", "ru": "Сохранить клиента", "uz": "Mijozni saqlash"},
	Description: "Saves a CRM client from the client forms.",
}, func(native.Deps) (native.CommandFunc[SaveClientInput, SaveClientOutput], error) {
	return func(ctx context.Context, c native.CommandContext, in SaveClientInput) (SaveClientOutput, error) {
		var out SaveClientOutput
		err := c.Transaction(ctx, func(ctx context.Context, tx native.Tx) (err error) {
			out, err = UpsertClient.Call(ctx, tx, in)
			return err
		})
		return out, err
	}, nil
}))

// ClientRef is the wire ref of a client.
func ClientRef(id uint) spi.ObjectRef {
	return spi.ObjectRef{Entity: ClientEntity, Key: strconv.FormatUint(uint64(id), 10)}
}

// ClientIDOf parses the client ID of a ref.
func ClientIDOf(ref spi.ObjectRef) (uint, error) {
	id, err := strconv.ParseUint(ref.Key, 10, 32)
	if err != nil || ref.Entity != ClientEntity {
		return 0, native.Validation(native.Issue{Code: IssueInvalidClientID, Path: []string{"client"}})
	}
	return uint(id), nil
}

func (s *ClientService) upsert(ctx context.Context, tx native.Tx, in SaveClientInput) (SaveClientOutput, error) {
	const op = serrors.Op("crm.ClientService.upsert")
	var existing client.Client
	if in.Client != nil {
		if err := tx.Authorize(crmPerms.ClientUpdate.Name()); err != nil {
			return SaveClientOutput{}, err
		}
		id, err := ClientIDOf(*in.Client)
		if err != nil {
			return SaveClientOutput{}, err
		}
		existing, err = s.repo.GetByID(ctx, id)
		if serrors.CodeOf(err) == serrors.NotFound {
			return SaveClientOutput{}, native.Validation(native.Issue{Code: IssueClientNotFound, Path: []string{"client"}})
		}
		if err != nil {
			return SaveClientOutput{}, serrors.Wrap(op, err)
		}
	} else if err := tx.Authorize(crmPerms.ClientCreate.Name()); err != nil {
		return SaveClientOutput{}, err
	}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return SaveClientOutput{}, serrors.Wrap(op, err)
	}
	data, err := in.apply(existing, tenant)
	if err != nil {
		return SaveClientOutput{}, err
	}
	var saved client.Client
	if existing == nil {
		saved, err = s.create(ctx, data)
	} else {
		saved, err = s.update(ctx, existing, data)
	}
	if err != nil {
		return SaveClientOutput{}, serrors.Wrap(op, err)
	}
	created := existing == nil
	if err := tx.AfterCommit(func(context.Context) { s.publishSaved(ctx, created, data, saved) }); err != nil {
		return SaveClientOutput{}, err
	}
	return SaveClientOutput{Client: ClientRef(saved.ID()), Created: created}, nil
}

// update saves data over before in the transaction of txCtx with its
// history record.
func (s *ClientService) update(txCtx context.Context, before, data client.Client) (client.Client, error) {
	updated, err := s.repo.Save(txCtx, data)
	if err != nil {
		return nil, err
	}
	if err := history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
		Type: ClientUpdated, Version: 1, Resource: ClientResource, ID: strconv.FormatUint(uint64(updated.ID()), 10),
		Before: clientSnapshot(before), After: clientSnapshot(updated),
	}); err != nil {
		return nil, err
	}
	return updated, nil
}

// publishSaved publishes the client event after the commit. The acting user
// and session are those of the request when there is one (a background
// trigger has neither).
func (s *ClientService) publishSaved(ctx context.Context, created bool, data, result client.Client) {
	sender, _ := ctx.Value(constants.UserKey).(user.User)
	sess, _ := ctx.Value(constants.SessionKey).(session.Session)
	if created {
		s.publisher.Publish(&client.CreatedEvent{Sender: sender, Session: sess, Data: data, Result: result})
		return
	}
	s.publisher.Publish(&client.UpdatedEvent{Sender: sender, Session: sess, Data: data, Result: result})
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func or(p *string, def string) string {
	if p == nil {
		return def
	}
	return *p
}

// apply builds the client to save: a new one from the input, or existing
// with the present fields applied (the semantics of the client forms).
func (in SaveClientInput) apply(existing client.Client, tenant uuid.UUID) (client.Client, error) {
	var issues []native.Issue
	invalid := func(code, path string) { issues = append(issues, native.Issue{Code: code, Path: []string{path}}) }
	var (
		ph  phone.Phone
		em  internet.Email
		pin tax.Pin
	)
	if v := str(in.Phone); v != "" {
		p, err := phone.NewFromE164(v)
		if err != nil {
			invalid(IssueInvalidPhone, "phone")
		}
		ph = p
	}
	if v := str(in.Email); v != "" {
		e, err := internet.NewEmail(v)
		if err != nil {
			invalid(IssueInvalidEmail, "email")
		}
		em = e
	}
	if v, cc := str(in.Pin), str(in.CountryCode); v != "" && cc != "" {
		c, err := country.New(cc)
		if err != nil {
			invalid(IssueInvalidCountry, "countryCode")
		} else if pin, err = tax.NewPin(v, c); err != nil {
			invalid(IssueInvalidPin, "pin")
		}
	}
	if len(issues) > 0 {
		return nil, native.Validation(issues...)
	}
	series, number := str(in.PassportSeries), str(in.PassportNumber)

	if existing == nil {
		opts := []client.Option{client.WithTenantID(tenant)}
		if v := str(in.Address); v != "" {
			opts = append(opts, client.WithAddress(v))
		}
		if ph != nil {
			opts = append(opts, client.WithPhone(ph))
		}
		if em != nil {
			opts = append(opts, client.WithEmail(em))
		}
		if series != "" && number != "" {
			opts = append(opts, client.WithPassport(passport.New(series, number)))
		}
		if pin != nil {
			opts = append(opts, client.WithPin(pin))
		}
		if v := str(in.LastName); v != "" {
			opts = append(opts, client.WithLastName(v))
		}
		if v := str(in.MiddleName); v != "" {
			opts = append(opts, client.WithMiddleName(v))
		}
		c, err := client.New(str(in.FirstName), opts...)
		if err != nil {
			return nil, err
		}
		if v := str(in.Comments); v != "" {
			c = c.SetComments(v)
		}
		return c, nil
	}

	c := existing
	if in.FirstName != nil || in.LastName != nil || in.MiddleName != nil {
		c = c.SetName(or(in.FirstName, c.FirstName()), or(in.LastName, c.LastName()), or(in.MiddleName, c.MiddleName()))
	}
	if ph != nil {
		c = c.SetPhone(ph)
	}
	if v := str(in.Address); v != "" {
		c = c.SetAddress(v)
	}
	if em != nil {
		c = c.SetEmail(em)
	}
	if series != "" && number != "" {
		var opts []passport.Option
		if p := existing.Passport(); p != nil && p.ID() != uuid.Nil {
			opts = append(opts, passport.WithID(p.ID()))
		}
		c = c.SetPassport(passport.New(series, number, opts...))
	}
	if pin != nil {
		c = c.SetPIN(pin)
	}
	if in.Comments != nil && *in.Comments != c.Comments() {
		c = c.SetComments(*in.Comments)
	}
	return c, nil
}

// DirectClientDispatcher runs the client contracts in-process over svc,
// authorizing with the request user's permissions: the dispatcher of an
// application without the extension runtime.
func DirectClientDispatcher(svc *ClientService) native.Dispatcher {
	return native.NewDirect(native.Bind(native.Default, native.StaticDeps(svc)), native.WithAuthorizer(native.UserAuthorizer))
}
