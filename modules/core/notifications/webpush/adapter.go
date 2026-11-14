// Package webpush is a notifications.ChannelAdapter for the push channel that
// speaks the Web Push protocol directly to browser push services: message
// encryption per RFC 8291 (aes128gcm, RFC 8188) and sender identification with
// VAPID (RFC 8292). Browsers register through PushSubscription; the
// subscription (endpoint plus keys) is the device token in the core push
// device registry.
//
// The adapter is not part of any default composition: an application builds it
// from keys it keeps in its secret store and installs it with
// core.ModuleOptions.NotificationWebPush. Only the Go standard library, the
// JWT library the SDK already depends on and net/http are used.
package webpush

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
)

// Urgency is the Urgency header of RFC 8030 section 5.3.
type Urgency string

const (
	UrgencyVeryLow Urgency = "very-low"
	UrgencyLow     Urgency = "low"
	UrgencyNormal  Urgency = "normal"
	UrgencyHigh    Urgency = "high"
)

// DefaultHosts are the endpoint host patterns of the push services of the
// major browsers. A pattern is an exact host or "*.suffix".
var DefaultHosts = []string{
	"fcm.googleapis.com",                // Chrome, Edge, Opera, Android browsers
	"updates.push.services.mozilla.com", // Firefox
	"*.push.services.mozilla.com",
	"web.push.apple.com", // Safari
	"*.push.apple.com",
	"*.notify.windows.com", // legacy Edge
}

// Options configures the adapter. Keys and subject come from the application's
// secret store or environment (see FromEnv); nothing is defaulted from code.
type Options struct {
	// PublicKey and PrivateKey are the VAPID key pair (see GenerateKeys).
	PublicKey  string
	PrivateKey string
	// Subject identifies the sender to push services: a mailto: or https: URL.
	Subject string
	// TTL is how long a push service keeps an undelivered message. Default 24h.
	TTL time.Duration
	// Urgency is the default message urgency. Default normal.
	Urgency Urgency
	// AllowedHosts limits the push service endpoints the adapter accepts and
	// connects to. Default DefaultHosts. It is a security boundary: a user
	// registers the endpoint, so without it a user could make the server send
	// requests to arbitrary hosts.
	AllowedHosts []string
	// InsecureAllowHTTP also accepts http:// endpoints. For tests against a
	// local push service only.
	InsecureAllowHTTP bool
	// HTTPClient sends the requests. Redirects are never followed. Default: a
	// client with a 20 second timeout.
	HTTPClient *http.Client
	// Now returns the current time (tests). Default time.Now.
	Now func() time.Time
}

// Adapter implements notifications.ChannelAdapter for browser push.
type Adapter struct {
	signer  *vapidSigner
	ttl     time.Duration
	urgency Urgency
	hosts   []string
	http    bool
	client  *http.Client
	now     func() time.Time
}

var _ notifications.ChannelAdapter = (*Adapter)(nil)

// New validates the options and returns the adapter.
func New(opts Options) (*Adapter, error) {
	signer, err := newVAPIDSigner(opts.PublicKey, opts.PrivateKey, opts.Subject)
	if err != nil {
		return nil, err
	}
	a := &Adapter{signer: signer, ttl: opts.TTL, urgency: opts.Urgency, hosts: opts.AllowedHosts, http: opts.InsecureAllowHTTP, client: opts.HTTPClient, now: opts.Now}
	if a.ttl == 0 {
		a.ttl = 24 * time.Hour
	}
	if a.ttl < 0 || a.ttl > 28*24*time.Hour {
		return nil, errors.New("webpush: TTL must be between 0 and 28 days")
	}
	switch a.urgency {
	case "":
		a.urgency = UrgencyNormal
	case UrgencyVeryLow, UrgencyLow, UrgencyNormal, UrgencyHigh:
	default:
		return nil, errors.New("webpush: unknown urgency")
	}
	if len(a.hosts) == 0 {
		a.hosts = DefaultHosts
	}
	if a.client == nil {
		a.client = &http.Client{Timeout: 20 * time.Second}
	}
	// A redirect would carry the request (and the VAPID token) to a host the allow-list never saw.
	c := *a.client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	a.client = &c
	if a.now == nil {
		a.now = time.Now
	}
	return a, nil
}

// Channel implements notifications.ChannelAdapter.
func (a *Adapter) Channel() notifications.Channel { return notifications.ChannelPush }

// PublicKey is the VAPID public key a page passes to pushManager.subscribe as
// applicationServerKey.
func (a *Adapter) PublicKey() string { return a.signer.public }

// ValidateSubscription checks a browser subscription before it is registered:
// its shape, its keys and that its endpoint is a permitted push service.
func (a *Adapter) ValidateSubscription(sub Subscription) error {
	u, err := sub.validate(a.http)
	if err != nil {
		return err
	}
	return a.checkHost(u)
}

func (a *Adapter) checkHost(u *url.URL) error {
	host := strings.ToLower(u.Hostname())
	for _, pattern := range a.hosts {
		pattern = strings.ToLower(pattern)
		if pattern == host || (strings.HasPrefix(pattern, "*.") && strings.HasSuffix(host, pattern[1:]) && len(host) > len(pattern)-1) {
			return nil
		}
	}
	return errors.New("push subscription endpoint is not a permitted push service")
}

// payload is the JSON document the service worker receives in its push event.
type payload struct {
	Title string            `json:"title,omitempty"`
	Body  string            `json:"body,omitempty"`
	URL   string            `json:"url,omitempty"`
	Data  map[string]string `json:"data,omitempty"`
	Tag   string            `json:"tag,omitempty"`
}

// Send implements notifications.ChannelAdapter. The message address is the
// device token (a Subscription as JSON).
//
//   - 201/202: accepted; the receipt is the Location of the message resource.
//   - 404/410: the subscription is gone; the failure is permanent and carries
//     notifications.ErrTargetGone so the core revokes the registered device.
//   - 400/401/403/413 and other 4xx: permanent, retrying cannot help.
//   - 408/429/5xx and transport errors: transient, retried by the core.
//   - a timeout after the request left: ambiguous.
//
// The delivery's idempotency key becomes the Topic header, so an offline
// device that receives a retried message shows it once.
func (a *Adapter) Send(ctx context.Context, m notifications.Message) (notifications.Receipt, error) {
	sub, err := ParseToken(m.Address)
	if err != nil {
		return notifications.Receipt{}, gone(err)
	}
	endpoint, err := sub.validate(a.http)
	if err != nil {
		return notifications.Receipt{}, gone(err)
	}
	if err := a.checkHost(endpoint); err != nil {
		return notifications.Receipt{}, permanent(err)
	}
	plaintext, err := json.Marshal(payload{Title: m.Title, Body: m.Body, URL: m.ActionURL, Data: m.Data, Tag: m.EventKey})
	if err != nil {
		return notifications.Receipt{}, permanent(err)
	}
	body, err := Encrypt(plaintext, sub)
	if err != nil {
		if errors.Is(err, errPayloadTooLarge) {
			return notifications.Receipt{}, permanent(err)
		}
		return notifications.Receipt{}, gone(err)
	}
	auth, err := a.signer.authorization(endpoint.Scheme+"://"+endpoint.Host, a.now())
	if err != nil {
		return notifications.Receipt{}, permanent(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(body))
	if err != nil {
		return notifications.Receipt{}, permanent(err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
	req.Header.Set("Authorization", auth)
	req.Header.Set("TTL", strconv.FormatInt(int64(a.ttl/time.Second), 10))
	req.Header.Set("Urgency", string(a.urgency))
	if topic := topicOf(m.IdempotencyKey); topic != "" {
		req.Header.Set("Topic", topic)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return notifications.Receipt{}, fmt.Errorf("%w: web push request timed out", notifications.ErrAmbiguous)
		}
		// The URL error would repeat the endpoint, which is a capability URL.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return notifications.Receipt{}, fmt.Errorf("web push request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	switch {
	case resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusAccepted:
		return notifications.Receipt{ProviderMessageID: resp.Header.Get("Location")}, nil
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return notifications.Receipt{}, gone(fmt.Errorf("push service answered %d", resp.StatusCode))
	case resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return notifications.Receipt{}, fmt.Errorf("push service answered %d", resp.StatusCode)
	default:
		return notifications.Receipt{}, permanent(fmt.Errorf("push service answered %d", resp.StatusCode))
	}
}

func permanent(err error) error { return fmt.Errorf("%w: %w", notifications.ErrPermanent, err) }

func gone(err error) error {
	return fmt.Errorf("%w: %w: %w", notifications.ErrPermanent, notifications.ErrTargetGone, err)
}

// topicOf maps an idempotency key to a Topic header value (RFC 8030 section
// 5.4: at most 32 characters of the URL-safe base64 alphabet).
func topicOf(key string) string {
	if key == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	return base64.RawURLEncoding.EncodeToString(sum[:])[:32]
}
