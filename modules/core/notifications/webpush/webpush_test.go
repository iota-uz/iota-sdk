package webpush

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
)

func b64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(s)
	require.NoError(t, err)
	return b
}

// RFC 8291 appendix A: the example message, byte for byte.
func TestEncryptMatchesRFC8291Vector(t *testing.T) {
	senderPrivate, err := ecdh.P256().NewPrivateKey(b64(t, "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"))
	require.NoError(t, err)
	require.Equal(t, b64(t, "BP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A8"), senderPrivate.PublicKey().Bytes())
	uaPublic, err := ecdh.P256().NewPublicKey(b64(t, "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4"))
	require.NoError(t, err)

	body, err := encrypt([]byte("When I grow up, I want to be a watermelon"), uaPublic, b64(t, "BTBZMqHH6r4Tts7J_aSIgg"), senderPrivate, b64(t, "DGv6ra1nlYgDCS1FRnbzlw"))
	require.NoError(t, err)
	require.Equal(t,
		"DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPTpK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN", base64.RawURLEncoding.EncodeToString(body))
}

// userAgent plays the browser: it owns the subscription keys and decrypts what the push service relays.
type userAgent struct {
	private *ecdh.PrivateKey
	auth    []byte
}

func newUserAgent(t *testing.T) (userAgent, Subscription) {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	require.NoError(t, err)
	auth := make([]byte, 16)
	_, err = io.ReadFull(rand.Reader, auth)
	require.NoError(t, err)
	return userAgent{private: key, auth: auth}, Subscription{Keys: SubscriptionKeys{
		P256DH: base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		Auth:   base64.RawURLEncoding.EncodeToString(auth),
	}}
}

// decrypt reverses RFC 8291 on the receiving side, independently of encrypt.
func (u userAgent) decrypt(t *testing.T, body []byte) []byte {
	t.Helper()
	require.GreaterOrEqual(t, len(body), headerLength+16)
	salt, rs, idlen := body[:16], binary.BigEndian.Uint32(body[16:20]), int(body[20])
	require.Equal(t, uint32(4096), rs)
	require.Equal(t, 65, idlen)
	senderPublicBytes := body[21 : 21+idlen]
	senderPublic, err := ecdh.P256().NewPublicKey(senderPublicBytes)
	require.NoError(t, err)
	secret, err := u.private.ECDH(senderPublic)
	require.NoError(t, err)
	prkKey, err := hkdf.Extract(sha256.New, secret, u.auth)
	require.NoError(t, err)
	info := append(append([]byte("WebPush: info\x00"), u.private.PublicKey().Bytes()...), senderPublicBytes...)
	ikm, err := hkdf.Expand(sha256.New, prkKey, string(info), 32)
	require.NoError(t, err)
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	require.NoError(t, err)
	cek, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	require.NoError(t, err)
	nonce, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	require.NoError(t, err)
	block, err := aes.NewCipher(cek)
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)
	record, err := aead.Open(nil, nonce, body[21+idlen:], nil)
	require.NoError(t, err)
	require.Equal(t, byte(0x02), record[len(record)-1], "the single record carries the final delimiter")
	return record[:len(record)-1]
}

// pushService is a local push service: it records requests and answers with a programmed status.
type pushService struct {
	srv    *httptest.Server
	mu     sync.Mutex
	status map[string]int
	seen   []*http.Request
	bodies [][]byte
}

func newPushService(t *testing.T) *pushService {
	t.Helper()
	p := &pushService{status: map[string]int{}}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		p.mu.Lock()
		p.seen, p.bodies = append(p.seen, r), append(p.bodies, body)
		status := p.status[r.URL.Path]
		p.mu.Unlock()
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "http://127.0.0.1:1/elsewhere", http.StatusTemporaryRedirect)
			return
		}
		if status == 0 {
			status = http.StatusCreated
		}
		if status == http.StatusCreated {
			w.Header().Set("Location", p.srv.URL+"/message/1")
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(p.srv.Close)
	return p
}

func (p *pushService) requests() ([]*http.Request, [][]byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]*http.Request(nil), p.seen...), append([][]byte(nil), p.bodies...)
}

func testAdapter(t *testing.T, svc *pushService, mutate ...func(*Options)) *Adapter {
	t.Helper()
	keys, err := GenerateKeys()
	require.NoError(t, err)
	opts := Options{PublicKey: keys.Public, PrivateKey: keys.Private, Subject: "mailto:ops@example.com", AllowedHosts: []string{"127.0.0.1"}, InsecureAllowHTTP: true}
	for _, m := range mutate {
		m(&opts)
	}
	a, err := New(opts)
	require.NoError(t, err)
	return a
}

func tokenFor(t *testing.T, sub Subscription, endpoint string) string {
	t.Helper()
	sub.Endpoint = endpoint
	token, err := sub.Token()
	require.NoError(t, err)
	return token
}

func message(token string) notifications.Message {
	return notifications.Message{
		DeliveryID: uuid.New(), IdempotencyKey: "delivery-1", Attempt: 1, Channel: notifications.ChannelPush, Address: token,
		EventKey: "acme.renewal.v1", Title: "Renewal", Body: "Policy P-1 expires", ActionURL: "/policies/P-1", Data: map[string]string{"policy": "P-1"},
	}
}

func TestSendDeliversAnEncryptedVAPIDSignedMessage(t *testing.T) {
	svc := newPushService(t)
	a := testAdapter(t, svc, func(o *Options) { o.TTL = 2 * time.Hour; o.Urgency = UrgencyHigh })
	ua, sub := newUserAgent(t)

	receipt, err := a.Send(context.Background(), message(tokenFor(t, sub, svc.srv.URL+"/push/abc")))
	require.NoError(t, err)
	require.Equal(t, svc.srv.URL+"/message/1", receipt.ProviderMessageID)

	reqs, bodies := svc.requests()
	require.Len(t, reqs, 1)
	r := reqs[0]
	require.Equal(t, "/push/abc", r.URL.Path)
	require.Equal(t, "aes128gcm", r.Header.Get("Content-Encoding"))
	require.Equal(t, "7200", r.Header.Get("TTL"))
	require.Equal(t, "high", r.Header.Get("Urgency"))
	require.Len(t, r.Header.Get("Topic"), 32)

	// the user agent can read what only it can decrypt
	var got payload
	require.NoError(t, json.Unmarshal(ua.decrypt(t, bodies[0]), &got))
	require.Equal(t, payload{Title: "Renewal", Body: "Policy P-1 expires", URL: "/policies/P-1", Data: map[string]string{"policy": "P-1"}, Tag: "acme.renewal.v1"}, got)

	// the VAPID header proves the sender: ES256 over the push service origin, verifiable with the public key
	auth := r.Header.Get("Authorization")
	require.True(t, strings.HasPrefix(auth, "vapid t="), auth)
	tokenPart, keyPart, ok := strings.Cut(strings.TrimPrefix(auth, "vapid t="), ", k=")
	require.True(t, ok)
	require.Equal(t, a.PublicKey(), keyPart)
	pub := b64(t, keyPart)
	verify := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(pub[1:33]), Y: new(big.Int).SetBytes(pub[33:])}
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(tokenPart, claims, func(*jwt.Token) (any, error) { return verify, nil }, jwt.WithValidMethods([]string{"ES256"}), jwt.WithAudience(svc.srv.URL))
	require.NoError(t, err)
	require.True(t, parsed.Valid)
	require.Equal(t, "mailto:ops@example.com", claims["sub"])
	exp, err := claims.GetExpirationTime()
	require.NoError(t, err)
	require.WithinDuration(t, exp.Time, time.Now().Add(vapidLifetime), time.Minute)

	// the same delivery always maps to the same topic, another delivery to another
	_, err = a.Send(context.Background(), message(tokenFor(t, sub, svc.srv.URL+"/push/abc")))
	require.NoError(t, err)
	other := message(tokenFor(t, sub, svc.srv.URL+"/push/abc"))
	other.IdempotencyKey = "delivery-2"
	_, err = a.Send(context.Background(), other)
	require.NoError(t, err)
	reqs, _ = svc.requests()
	require.Equal(t, reqs[0].Header.Get("Topic"), reqs[1].Header.Get("Topic"))
	require.NotEqual(t, reqs[0].Header.Get("Topic"), reqs[2].Header.Get("Topic"))
}

func TestSendClassifiesPushServiceAnswers(t *testing.T) {
	svc := newPushService(t)
	a := testAdapter(t, svc)
	_, sub := newUserAgent(t)
	svc.status["/gone"], svc.status["/expired"] = http.StatusGone, http.StatusNotFound
	svc.status["/bad"], svc.status["/big"] = http.StatusBadRequest, http.StatusRequestEntityTooLarge
	svc.status["/forbidden"] = http.StatusForbidden
	svc.status["/busy"], svc.status["/down"] = http.StatusTooManyRequests, http.StatusBadGateway
	send := func(path string) error {
		_, err := a.Send(context.Background(), message(tokenFor(t, sub, svc.srv.URL+path)))
		return err
	}

	for _, path := range []string{"/gone", "/expired"} {
		err := send(path)
		require.ErrorIs(t, err, notifications.ErrPermanent, path)
		require.ErrorIs(t, err, notifications.ErrTargetGone, path)
	}
	for _, path := range []string{"/bad", "/big", "/forbidden"} {
		err := send(path)
		require.ErrorIs(t, err, notifications.ErrPermanent, path)
		require.NotErrorIs(t, err, notifications.ErrTargetGone, "a rejected message does not mean the device is gone: "+path)
	}
	for _, path := range []string{"/busy", "/down"} {
		err := send(path)
		require.Error(t, err, path)
		require.NotErrorIs(t, err, notifications.ErrPermanent, "retryable: "+path)
		require.NotErrorIs(t, err, notifications.ErrAmbiguous, path)
	}

	// a redirect is a refusal, not something to follow to a host nobody allowed
	err := send("/redirect")
	require.ErrorIs(t, err, notifications.ErrPermanent)
}

func TestSendTimeoutIsAmbiguous(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body) // lets the server notice the client giving up
		<-r.Context().Done()
	}))
	defer slow.Close()
	a := testAdapter(t, nil)
	_, sub := newUserAgent(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := a.Send(ctx, message(tokenFor(t, sub, slow.URL+"/push/x")))
	require.ErrorIs(t, err, notifications.ErrAmbiguous)
}

func TestSendRefusesWhatItMustNotSend(t *testing.T) {
	svc := newPushService(t)
	a := testAdapter(t, svc)
	_, sub := newUserAgent(t)

	// a token that is not a subscription can never work: permanent and gone
	_, err := a.Send(context.Background(), message("not-a-subscription"))
	require.ErrorIs(t, err, notifications.ErrPermanent)
	require.ErrorIs(t, err, notifications.ErrTargetGone)

	// an endpoint outside the allow-list is never contacted
	closed := testAdapter(t, svc, func(o *Options) { o.AllowedHosts = []string{"fcm.googleapis.com"} })
	_, err = closed.Send(context.Background(), message(tokenFor(t, sub, svc.srv.URL+"/push/abc")))
	require.ErrorIs(t, err, notifications.ErrPermanent)
	reqs, _ := svc.requests()
	require.Empty(t, reqs)

	// a payload that does not fit one record is rejected before any request
	big := message(tokenFor(t, sub, svc.srv.URL+"/push/abc"))
	big.Body = strings.Repeat("x", MaxPayload)
	_, err = a.Send(context.Background(), big)
	require.ErrorIs(t, err, notifications.ErrPermanent)
	require.NotErrorIs(t, err, notifications.ErrTargetGone)
	reqs, _ = svc.requests()
	require.Empty(t, reqs)
}

func TestSubscriptionValidation(t *testing.T) {
	svc := newPushService(t)
	_, sub := newUserAgent(t)
	strict, err := New(mustOptions(t))
	require.NoError(t, err)
	sub.Endpoint = "https://fcm.googleapis.com/fcm/send/abc"
	require.NoError(t, strict.ValidateSubscription(sub))
	sub.Endpoint = "https://updates.push.services.mozilla.com/wpush/v2/abc"
	require.NoError(t, strict.ValidateSubscription(sub))
	sub.Endpoint = "https://web.push.apple.com/abc"
	require.NoError(t, strict.ValidateSubscription(sub))
	for _, bad := range []string{
		"http://fcm.googleapis.com/x", "https://evil.example.com/x", "https://fcm.googleapis.com.evil.example/x",
		"https://user:pw@fcm.googleapis.com/x", "https://169.254.169.254/latest", "https://localhost/x", "fcm.googleapis.com", "",
		svc.srv.URL,
	} {
		sub.Endpoint = bad
		require.Error(t, strict.ValidateSubscription(sub), bad)
	}
	sub.Endpoint = "https://fcm.googleapis.com/x"
	sub.Keys.Auth = "short"
	require.Error(t, strict.ValidateSubscription(sub))
	sub.Keys.Auth, sub.Keys.P256DH = "BTBZMqHH6r4Tts7J_aSIgg", "AAAA"
	require.Error(t, strict.ValidateSubscription(sub))
	require.NotEqual(t, DeviceID(Subscription{Endpoint: "https://fcm.googleapis.com/a"}), DeviceID(Subscription{Endpoint: "https://fcm.googleapis.com/b"}))
	require.Equal(t, DeviceID(Subscription{Endpoint: "https://fcm.googleapis.com/a"}), DeviceID(Subscription{Endpoint: "https://fcm.googleapis.com/a", Keys: SubscriptionKeys{Auth: "x"}}))
}

func mustOptions(t *testing.T) Options {
	t.Helper()
	keys, err := GenerateKeys()
	require.NoError(t, err)
	return Options{PublicKey: keys.Public, PrivateKey: keys.Private, Subject: "https://example.com/contact"}
}

func TestNewRejectsBadConfiguration(t *testing.T) {
	good := mustOptions(t)
	other, err := GenerateKeys()
	require.NoError(t, err)
	for name, mutate := range map[string]func(*Options){
		"mismatched keys":   func(o *Options) { o.PublicKey = other.Public },
		"empty private key": func(o *Options) { o.PrivateKey = "" },
		"garbage key":       func(o *Options) { o.PrivateKey = "%%%" },
		"no subject":        func(o *Options) { o.Subject = "" },
		"plain subject":     func(o *Options) { o.Subject = "ops@example.com" },
		"negative ttl":      func(o *Options) { o.TTL = -time.Second },
		"unknown urgency":   func(o *Options) { o.Urgency = "asap" },
	} {
		opts := good
		mutate(&opts)
		_, err := New(opts)
		require.Error(t, err, name)
	}
	_, err = New(good)
	require.NoError(t, err)
}

func TestFromEnvIsOffUnlessConfigured(t *testing.T) {
	env := func(m map[string]string) func(string) (string, bool) {
		return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
	}
	_, ok, err := FromEnv(env(nil))
	require.NoError(t, err)
	require.False(t, ok, "web push is off by default")

	_, _, err = FromEnv(env(map[string]string{EnvPublicKey: "pub"}))
	require.Error(t, err, "a half-configured deployment fails loudly")

	opts, ok, err := FromEnv(env(map[string]string{EnvPublicKey: "pub", EnvPrivateKey: "priv", EnvSubject: "mailto:a@b.c", EnvHosts: "fcm.googleapis.com, *.push.apple.com", EnvTTL: "6h"}))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, Options{PublicKey: "pub", PrivateKey: "priv", Subject: "mailto:a@b.c", AllowedHosts: []string{"fcm.googleapis.com", "*.push.apple.com"}, TTL: 6 * time.Hour}, opts)

	_, _, err = FromEnv(env(map[string]string{EnvPublicKey: "pub", EnvPrivateKey: "priv", EnvSubject: "mailto:a@b.c", EnvTTL: "soon"}))
	require.Error(t, err)
}
