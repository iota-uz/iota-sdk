package smstest

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// Eskiz is a fake of the Eskiz API: login, send and the balance.
type Eskiz struct {
	base
	Email, Password string

	// LoginScript, when set, answers the n-th login request (1 for the first) instead of the fake; a
	// Reply with Status 0 keeps the fake's own answer.
	LoginScript func(n int) Reply

	lmu      sync.Mutex
	loginReq int
	logins   int
	requests int
	tokens   map[string]bool
}

// NewEskiz starts a fake that accepts the given account.
func NewEskiz(email, password string) *Eskiz {
	e := &Eskiz{Email: email, Password: password, tokens: map[string]bool{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", e.login)
	mux.HandleFunc("/api/message/sms/send", e.send)
	mux.HandleFunc("/api/user/get-limit", e.limit)
	e.init(mux)
	return e
}

// Logins is how many times a client logged in.
func (e *Eskiz) Logins() int {
	e.lmu.Lock()
	defer e.lmu.Unlock()
	return e.logins
}

// ExpireTokens makes every token issued so far unauthorized (the next request is a 401).
func (e *Eskiz) ExpireTokens() {
	e.lmu.Lock()
	e.tokens = map[string]bool{}
	e.lmu.Unlock()
}

func (e *Eskiz) login(w http.ResponseWriter, r *http.Request) {
	e.lmu.Lock()
	e.loginReq++
	n, script := e.loginReq, e.LoginScript
	e.lmu.Unlock()
	if script != nil {
		if rep := script(n); rep.Status != 0 {
			e.write(w, r, rep, rep.Status, "")
			return
		}
	}
	_ = r.ParseMultipartForm(1 << 20)
	if r.FormValue("email") != e.Email || r.FormValue("password") != e.Password {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"Unauthorized"}`)
		return
	}
	e.lmu.Lock()
	e.logins++
	tok := fmt.Sprintf("tok-%d", e.logins)
	e.tokens[tok] = true
	e.lmu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"message":"token_generated","data":{"token":%q},"token_type":"bearer"}`, tok)
}

func (e *Eskiz) authorized(w http.ResponseWriter, r *http.Request) bool {
	e.lmu.Lock()
	ok := e.tokens[bearer(r)]
	e.lmu.Unlock()
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"Expired"}`)
	}
	return ok
}

func (e *Eskiz) limit(w http.ResponseWriter, r *http.Request) {
	if !e.authorized(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"data":{"balance":100},"status":"success"}`)
}

// SendRequests is every request the send endpoint got, refused ones included.
func (e *Eskiz) SendRequests() int {
	e.lmu.Lock()
	defer e.lmu.Unlock()
	return e.requests
}

func (e *Eskiz) send(w http.ResponseWriter, r *http.Request) {
	e.lmu.Lock()
	e.requests++
	e.lmu.Unlock()
	if !e.authorized(w, r) {
		return
	}
	_ = r.ParseMultipartForm(1 << 20)
	phone := r.FormValue("mobile_phone")
	s, reply := e.record(Sent{To: phone, From: r.FormValue("from"), Text: r.FormValue("message")})
	if reply.Status == 0 && !reply.Hang && (len(phone) != 12 || strings.TrimLeft(phone, "0123456789") != "") {
		reply = Reply{Status: http.StatusBadRequest, Body: `{"message":"Invalid mobile phone"}`}
	}
	e.write(w, r, reply, http.StatusOK, fmt.Sprintf(`{"id":"eskiz-msg-%d","message":"Waiting for SMS provider","status":"waiting"}`, s.At))
}

// Transport is a RoundTripper that sends every request to this fake (for a connector built with
// sms.WithTransport); the Eskiz connector takes the address from its credentials instead.
func (e *Eskiz) Transport() http.RoundTripper { return newRedirect(e.URL()) }
