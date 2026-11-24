package smstest

import (
	"fmt"
	"net/http"
	"strings"
)

// Twilio is a fake of the Twilio REST API: creating a message and reading the account.
type Twilio struct {
	base
	AccountSID, AuthToken string
}

// NewTwilio starts a fake that accepts the given account.
func NewTwilio(sid, token string) *Twilio {
	t := &Twilio{AccountSID: sid, AuthToken: token}
	mux := http.NewServeMux()
	mux.HandleFunc("/2010-04-01/Accounts/", t.handle)
	t.init(mux)
	return t
}

// Transport is a RoundTripper that sends every request to this fake: twilio-go has no base URL option.
func (t *Twilio) Transport() http.RoundTripper { return newRedirect(t.URL()) }

func (t *Twilio) handle(w http.ResponseWriter, r *http.Request) {
	user, pass, _ := r.BasicAuth()
	if user != t.AccountSID || pass != t.AuthToken {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"code":20003,"message":"Authenticate","more_info":"https://www.twilio.com/docs/errors/20003","status":401}`)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/2010-04-01/Accounts/")
	switch {
	case r.Method == http.MethodPost && path == t.AccountSID+"/Messages.json":
		_ = r.ParseForm()
		s, reply := t.record(Sent{To: r.PostForm.Get("To"), From: r.PostForm.Get("From"), Text: r.PostForm.Get("Body"), Service: r.PostForm.Get("MessagingServiceSid")})
		if reply.Status == 0 && !reply.Hang && !strings.HasPrefix(s.To, "+") {
			reply = Reply{Status: http.StatusBadRequest, Body: `{"code":21211,"message":"The 'To' number is not a valid phone number.","status":400}`}
		}
		t.write(w, r, reply, http.StatusCreated, fmt.Sprintf(`{"sid":"SM%030d","status":"queued","num_segments":"2","to":%q}`, s.At, s.To))
	case r.Method == http.MethodGet && path == t.AccountSID+".json":
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"sid":%q,"friendly_name":"fake","status":"active"}`, t.AccountSID)
	default:
		http.NotFound(w, r)
	}
}
