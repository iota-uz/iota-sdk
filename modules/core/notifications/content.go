package notifications

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
	"sync"
)

// ChannelContent is the rendered message of one notice for one channel and
// language. Which fields a channel uses:
//
//   - in_app: Title, Body, ActionURL
//   - push: Title, Body, ActionURL and Data (custom key/value payload)
//   - email: Subject, HTML, Text
//   - sms: Text
//
// Fields a channel does not use are ignored. The persisted delivery keeps the
// complete content, so a queued delivery is sent exactly as rendered.
type ChannelContent struct {
	Title     string            `json:"title,omitempty"`
	Body      string            `json:"body,omitempty"`
	ActionURL string            `json:"action_url,omitempty"`
	Subject   string            `json:"subject,omitempty"`
	HTML      string            `json:"html,omitempty"`
	Text      string            `json:"text,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
}

// Summary returns the title and body persisted in the delivery's own columns:
// the email subject and text body, or the SMS text.
func (c ChannelContent) Summary(channel Channel) (string, string) {
	switch channel {
	case ChannelEmail:
		return firstNonEmpty(c.Subject, c.Title), firstNonEmpty(c.Text, c.Body)
	case ChannelSMS:
		return "", firstNonEmpty(c.Text, c.Body)
	case ChannelInApp, ChannelPush:
		return c.Title, c.Body
	}
	return c.Title, c.Body
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// RenderRequest asks for the content of one channel. Event carries the payload,
// the definition version and the snapshot the notice was pinned to.
type RenderRequest struct {
	Event    Event
	Channel  Channel
	Language string
}

// ErrUnknownDefinition is returned when no renderer or definition exists for
// the pinned notice.
var ErrUnknownDefinition = errors.New("notification definition not found")

// ContentRenderer renders a notice for one channel. Workers call it after the
// producer's transaction committed and never inside a database transaction
// owned by the producer. Wrap ErrPermanent for a failure that retrying cannot
// fix.
type ContentRenderer interface {
	Render(ctx context.Context, request RenderRequest) (ChannelContent, error)
}

// ContentRendererFunc adapts a function to ContentRenderer.
type ContentRendererFunc func(context.Context, RenderRequest) (ChannelContent, error)

// Render implements ContentRenderer.
func (f ContentRendererFunc) Render(ctx context.Context, request RenderRequest) (ChannelContent, error) {
	return f(ctx, request)
}

// ContentRenderers resolves the renderer of a notice by event key and snapshot
// reference. Renderers registered for an exact (key, snapshot) pin win, then a
// renderer registered for the key alone, then the native definition renderer of
// the catalog (the Render function and ActionURL builder of the definition).
// This is the plug-in point for runtime (TypeScript) templates.
type ContentRenderers struct {
	catalog *Catalog
	mu      sync.RWMutex
	pinned  map[string]ContentRenderer
}

// NewContentRenderers returns a registry that falls back to the catalog.
func NewContentRenderers(catalog *Catalog) *ContentRenderers {
	return &ContentRenderers{catalog: catalog, pinned: map[string]ContentRenderer{}}
}

func rendererID(eventKey, snapshotRef string) string { return eventKey + "@" + snapshotRef }

// Register binds a renderer to an event key and snapshot reference (empty for
// all snapshots of the key). A second registration of the same pin fails
// instead of replacing the renderer that queued deliveries rely on.
func (r *ContentRenderers) Register(eventKey, snapshotRef string, renderer ContentRenderer) error {
	if strings.TrimSpace(eventKey) == "" || renderer == nil {
		return errors.New("invalid notification content renderer")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	id := rendererID(eventKey, snapshotRef)
	if _, exists := r.pinned[id]; exists {
		return fmt.Errorf("notification content renderer already registered: %s", id)
	}
	r.pinned[id] = renderer
	return nil
}

func (r *ContentRenderers) lookup(event Event) (ContentRenderer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if renderer, ok := r.pinned[rendererID(event.Key, event.SnapshotRef)]; ok {
		return renderer, true
	}
	if event.SnapshotRef != "" {
		return nil, false
	}
	renderer, ok := r.pinned[rendererID(event.Key, "")]
	return renderer, ok
}

// Render renders the notice with the renderer pinned to its snapshot.
func (r *ContentRenderers) Render(ctx context.Context, request RenderRequest) (ChannelContent, error) {
	if renderer, ok := r.lookup(request.Event); ok {
		return renderer.Render(ctx, request)
	}
	return r.native(request)
}

// native renders through the catalog definition resolved for the event's
// snapshot, so definitions deactivated after queueing still render.
func (r *ContentRenderers) native(request RenderRequest) (ChannelContent, error) {
	definition, ok := r.catalog.For(request.Event)
	if !ok {
		return ChannelContent{}, ErrUnknownDefinition
	}
	language := request.Language
	if language == "" {
		language = "en"
	}
	rendered, err := definition.Render(request.Event, language)
	if err != nil {
		return ChannelContent{}, err
	}
	if definition.ActionURL != nil {
		rendered.ActionURL, err = definition.ActionURL(request.Event)
		if err != nil {
			return ChannelContent{}, err
		}
	}
	return contentForChannel(request.Channel, rendered), nil
}

// contentForChannel adapts a definition's title/body to the shape a channel needs.
func contentForChannel(channel Channel, rendered Content) ChannelContent {
	content := ChannelContent{Title: rendered.Title, Body: rendered.Body, ActionURL: rendered.ActionURL}
	switch channel {
	case ChannelEmail:
		content.Subject = rendered.Title
		content.Text = rendered.Body
		content.HTML = "<p>" + html.EscapeString(rendered.Body) + "</p>"
	case ChannelSMS:
		content.Text = strings.TrimSpace(rendered.Title + "\n" + rendered.Body)
	case ChannelInApp, ChannelPush:
		// Title and body already carry everything these channels show.
	}
	return content
}
