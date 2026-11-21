package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
	"github.com/iota-uz/iota-sdk/pkg/listext"
)

// bind implements the declared method name with fn.
func bind[Req, Resp any](name string, fn func(context.Context, Req) (Resp, error)) solid.Binding {
	for _, m := range screen.Methods() {
		if m.Name == name {
			return solid.Bind(m, fn)
		}
	}
	panic(fmt.Sprintf("integrations: RPC method %q is not declared on the screen", name))
}

func (c *Controller) bindings() []solid.Binding {
	return []solid.Binding{
		bind(MethodList, c.list),
		bind(MethodUsage, c.usage),
		bind(MethodCreate, c.create),
		bind(MethodOAuthStart, c.oauthStart),
		bind(MethodUpdate, c.update),
		bind(MethodTest, c.test),
		bind(MethodSetDisabled, c.setDisabled),
		bind(MethodDelete, c.delete),
		bind(MethodPlatform, c.setPlatform),
	}
}

var manage = permissions.ConnectionsManage.Name()

func (c *Controller) list(ctx context.Context, _ Empty) (ListResponse, error) {
	sub, err := caller(ctx, manage, permissions.AutomationsAuthor.Name())
	if err != nil {
		return ListResponse{}, err
	}
	canManage := sub.Has(manage)
	entries, err := c.cfg.Service.List(ctx, actor(sub), services.ListOptions{WithFields: canManage})
	if err != nil {
		return ListResponse{}, err
	}
	lang := listext.LangFromContext(ctx)
	out := ListResponse{Connectors: make([]ConnectorView, 0, len(entries)), CanManage: canManage}
	for _, e := range entries {
		out.Connectors = append(out.Connectors, connectorView(e, lang, canManage))
	}
	return out, nil
}

func connectorView(e services.ConnectorEntry, lang string, canManage bool) ConnectorView {
	d := e.Connector.Descriptor()
	kind := string(d.Kind)
	category := Category(d.Category)
	switch category {
	case CategoryMessaging, CategoryEmail, CategoryAI, CategoryDocuments, CategoryOther:
	default:
		category = CategoryOther
	}
	v := ConnectorView{
		Kind:             kind,
		Title:            listext.LabelFor(d.Title, lang, kind),
		Icon:             d.Icon,
		Description:      listext.LabelFor(d.Description, lang, ""),
		Category:         category,
		Auth:             AuthKind(d.Auth),
		CredentialSchema: localizeSchema(d.Credentials, lang),
		Nodes:            nodeTitles(e.Connector, lang),
		Connections:      make([]ConnectionView, 0, len(e.Connections)),
	}
	if e.Platform != nil {
		v.Platform = &PlatformView{Policy: PlatformPolicy(e.Platform.Policy), Available: e.Platform.Available, EnabledForTenant: e.Platform.EnabledForTenant}
	}
	for _, conn := range e.Connections {
		cv := connectionView(conn)
		if !canManage {
			cv.Fields = nil
		}
		v.Connections = append(v.Connections, cv)
	}
	return v
}

func nodeTitles(conn connectors.Connector, lang string) []string {
	out := []string{}
	for _, n := range conn.Nodes() {
		spec := n.Spec()
		if l, ok := n.(catalog.LocalizedNode); ok {
			spec = l.Localize(spec, lang)
		}
		title := spec.Title
		if title == "" {
			title = spec.Type
		}
		out = append(out, title)
	}
	return out
}

// localizeSchema is the credentials schema with "x-label" / "x-description" (a label by language)
// resolved into "title" / "description" for lang.
func localizeSchema(raw json.RawMessage, lang string) any {
	if len(raw) == 0 {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	localizeNode(doc, lang)
	if props, ok := doc["properties"].(map[string]any); ok {
		for _, p := range props {
			if pm, ok := p.(map[string]any); ok {
				localizeNode(pm, lang)
			}
		}
	}
	return doc
}

func localizeNode(n map[string]any, lang string) {
	for src, dst := range map[string]string{"x-label": "title", "x-description": "description"} {
		labels, ok := n[src].(map[string]any)
		if !ok {
			continue
		}
		l := map[string]string{}
		for k, v := range labels {
			if s, ok := v.(string); ok {
				l[k] = s
			}
		}
		if s := listext.LabelFor(l, lang, ""); s != "" {
			n[dst] = s
		}
		delete(n, src)
	}
}

func connectionView(e services.ConnectionEntry) ConnectionView {
	c := e.Connection
	v := ConnectionView{
		ID: c.ID.String(), Kind: string(c.Kind), Name: c.Name, DisplayName: c.DisplayName, Version: c.Version,
		Status: Status(c.Status), UsedBy: e.UsedBy,
		CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: c.UpdatedAt.UTC().Format(time.RFC3339),
		Fields: e.Fields,
	}
	if c.Identity.Display != "" {
		s := c.Identity.Display
		v.Identity = &s
	}
	if c.StatusReason != "" {
		s := c.StatusReason
		v.StatusReason = &s
	}
	if e.LastChecked != nil {
		s := e.LastChecked.UTC().Format(time.RFC3339)
		v.LastChecked = &s
	}
	return v
}

func usageViews(uses []connectors.Use) []UsageView {
	out := make([]UsageView, 0, len(uses))
	for _, u := range uses {
		nodes := u.NodeIDs
		if nodes == nil {
			nodes = []string{}
		}
		out = append(out, UsageView{AutomationID: u.AutomationID.String(), Name: u.Name, Version: u.Version, Enabled: u.Enabled, NodeIDs: nodes})
	}
	return out
}

func (c *Controller) usage(ctx context.Context, req IDRequest) (UsageResponse, error) {
	sub, err := caller(ctx, manage)
	if err != nil {
		return UsageResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return UsageResponse{}, err
	}
	uses, err := c.cfg.Service.ConnectionUsage(ctx, actor(sub), id)
	if err != nil {
		return UsageResponse{}, mapErr(err)
	}
	return UsageResponse{Usages: usageViews(uses)}, nil
}

// saveResult turns a refusal of create or update into the screen's result; other errors are RPC
// errors.
func saveResult(entry services.ConnectionEntry, err error) (SaveResponse, error) {
	if err == nil {
		v := connectionView(entry)
		return SaveResponse{OK: true, Connection: &v}, nil
	}
	var (
		invalid *services.InvalidInputError
		failed  *services.TestFailedError
	)
	switch {
	case errors.As(err, &invalid):
		return SaveResponse{Error: ErrInvalidInput, Fields: invalid.Fields}, nil
	case errors.As(err, &failed):
		return SaveResponse{Error: ErrTestFailed, Message: failed.Message}, nil
	case errors.Is(err, connectors.ErrNameTaken):
		return SaveResponse{Error: ErrNameTaken, Fields: map[string]string{"name": "taken"}}, nil
	case errors.Is(err, connectors.ErrVersionConflict):
		return SaveResponse{Error: ErrVersionConflict}, nil
	}
	return SaveResponse{}, mapErr(err)
}

func (c *Controller) create(ctx context.Context, req CreateRequest) (SaveResponse, error) {
	sub, err := caller(ctx, manage)
	if err != nil {
		return SaveResponse{}, err
	}
	return saveResult(c.cfg.Service.Create(ctx, actor(sub), services.CreateInput{
		Kind: kindOf(req.Kind), Name: req.Name, DisplayName: req.DisplayName, Credentials: req.Credentials,
	}))
}

func (c *Controller) update(ctx context.Context, req UpdateRequest) (SaveResponse, error) {
	sub, err := caller(ctx, manage)
	if err != nil {
		return SaveResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return SaveResponse{}, err
	}
	return saveResult(c.cfg.Service.Update(ctx, actor(sub), services.UpdateInput{
		ID: id, Version: req.Version, DisplayName: req.DisplayName, Credentials: req.Credentials,
	}))
}

func (c *Controller) test(ctx context.Context, req IDRequest) (TestResponse, error) {
	sub, err := caller(ctx, manage)
	if err != nil {
		return TestResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return TestResponse{}, err
	}
	res, err := c.cfg.Service.Test(ctx, actor(sub), id)
	if err != nil {
		return TestResponse{}, mapErr(err)
	}
	if res.NeedsReauth {
		return TestResponse{Error: ErrNeedsReauth}, nil
	}
	return TestResponse{OK: res.OK, Identity: res.Identity, Message: res.Message}, nil
}

func (c *Controller) setDisabled(ctx context.Context, req SetDisabledRequest) (ConnectionResponse, error) {
	sub, err := caller(ctx, manage)
	if err != nil {
		return ConnectionResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return ConnectionResponse{}, err
	}
	e, err := c.cfg.Service.SetDisabled(ctx, actor(sub), id, req.Disabled)
	if err != nil {
		return ConnectionResponse{}, mapErr(err)
	}
	return ConnectionResponse{Connection: connectionView(e)}, nil
}

func (c *Controller) delete(ctx context.Context, req IDRequest) (DeleteResponse, error) {
	sub, err := caller(ctx, manage)
	if err != nil {
		return DeleteResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return DeleteResponse{}, err
	}
	err = c.cfg.Service.Delete(ctx, actor(sub), id)
	var inUse *connectors.InUseError
	switch {
	case err == nil:
		return DeleteResponse{OK: true}, nil
	case errors.As(err, &inUse):
		return DeleteResponse{Error: ErrConnectionInUse, Usages: usageViews(inUse.Uses)}, nil
	}
	return DeleteResponse{}, mapErr(err)
}

func (c *Controller) setPlatform(ctx context.Context, req PlatformRequest) (OKResponse, error) {
	sub, err := caller(ctx, manage)
	if err != nil {
		return OKResponse{}, err
	}
	if err := c.cfg.Service.SetPlatformEnabled(ctx, actor(sub), kindOf(req.Kind), req.Enabled); err != nil {
		return OKResponse{}, mapErr(err)
	}
	return OKResponse{OK: true}, nil
}

func (c *Controller) oauthStart(ctx context.Context, req OAuthStartRequest) (OAuthStartResponse, error) {
	sub, err := caller(ctx, manage)
	if err != nil {
		return OAuthStartResponse{}, err
	}
	connID := uuid.Nil
	if req.ConnectionID != "" {
		if connID, err = parseID(req.ConnectionID); err != nil {
			return OAuthStartResponse{}, err
		}
	}
	r := requestFromContext(ctx)
	redirect := c.cfg.Origin + c.callbackPath()
	if r != nil {
		redirect = c.redirectURL(r)
	}
	authURL, err := c.cfg.Service.OAuthStart(ctx, actor(sub), kindOf(req.Kind), req.Name, connID, redirect)
	if err != nil {
		return OAuthStartResponse{}, mapErr(err)
	}
	return OAuthStartResponse{AuthorizeURL: authURL}, nil
}

// mapErr turns service errors into RPC errors the screen explains; anything else is internal.
func mapErr(err error) error {
	var invalid *services.InvalidInputError
	switch {
	case errors.Is(err, connectors.ErrNotFound):
		return notFound()
	case errors.Is(err, connectors.ErrNotConfigured), errors.Is(err, services.ErrOAuthUnavailable):
		return solid.NewRPCError(string(ErrUnavailable), "connections are not configured in this application", map[string]string{"code": string(ErrUnavailable)})
	case errors.As(err, &invalid):
		return solid.NewRPCError(solid.CodeFieldValidation, "invalid input", invalid.Fields)
	}
	return err
}
