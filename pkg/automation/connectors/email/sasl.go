package email

import (
	"bytes"
	"encoding/base64"
	"io"
	"regexp"
	"strings"
	"sync"

	"github.com/emersion/go-sasl"
)

// xoauth2 is the SASL XOAUTH2 mechanism of Gmail and Microsoft 365: an initial response that carries the
// mailbox and a bearer token. A server that refuses the token answers with a JSON challenge; the client
// answers it with an empty response and the server then ends the exchange with a failure.
type xoauth2 struct{ user, token string }

func newXOAuth2(user, token string) sasl.Client { return &xoauth2{user: user, token: token} }

func (x *xoauth2) Start() (string, []byte, error) {
	return "XOAUTH2", []byte("user=" + x.user + "\x01auth=Bearer " + x.token + "\x01\x01"), nil
}

func (x *xoauth2) Next([]byte) ([]byte, error) { return []byte{}, nil }

// ---------------------------------------------------------------------------------------------
// the dialogue log

// maskedWriter writes the dialogue with a server to w with no secret in it: every AUTH exchange is cut to
// its command, and every credential value (and its base64 form, the one SASL carries) is replaced.
//
// The client writes both directions to one writer, as lines; a line that carries a secret is not always
// recognisable by its command (the continuation of AUTHENTICATE is a bare base64 line), so the values are
// replaced wherever they appear and the base64 lines that follow an AUTH command are masked whole.
type maskedWriter struct {
	mu      sync.Mutex
	w       io.Writer
	secrets []string
	buf     bytes.Buffer
	inAuth  bool
}

var authLine = regexp.MustCompile(`(?i)^(?:[A-Za-z0-9.*]+ )?(AUTH(?:ENTICATE)?|LOGIN)\b`)

const maskText = "***"

// newMaskedWriter is the writer for a dialogue with credentials values: empty ones are ignored.
func newMaskedWriter(w io.Writer, values ...string) io.Writer {
	if w == nil {
		return nil
	}
	var secrets []string
	for _, v := range values {
		if v == "" {
			continue
		}
		secrets = append(secrets, v, base64.StdEncoding.EncodeToString([]byte(v)), base64.RawStdEncoding.EncodeToString([]byte(v)))
	}
	return &maskedWriter{w: w, secrets: secrets}
}

func (m *maskedWriter) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buf.Write(p)
	for {
		i := bytes.IndexByte(m.buf.Bytes(), '\n')
		if i < 0 {
			break
		}
		line := string(m.buf.Next(i + 1))
		if _, err := io.WriteString(m.w, m.mask(line)); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (m *maskedWriter) mask(line string) string {
	trimmed := strings.TrimRight(line, "\r\n")
	switch {
	case authLine.MatchString(trimmed):
		// Keep the command and the mechanism, drop whatever follows (an initial response, a password).
		fields := strings.Fields(trimmed)
		keep := 1
		if len(fields) > 1 && !authLine.MatchString(fields[0]) {
			keep = 2 // a tag, then the command
		}
		for keep < len(fields) && keep < 3 && isMechanism(fields[keep]) {
			keep++
		}
		m.inAuth = true
		return strings.Join(fields[:min(keep, len(fields))], " ") + " " + maskText + "\r\n"
	case m.inAuth && isBase64Line(trimmed):
		return maskText + "\r\n"
	}
	m.inAuth = false
	for _, s := range m.secrets {
		trimmed = strings.ReplaceAll(trimmed, s, maskText)
	}
	return trimmed + "\r\n"
}

func isMechanism(s string) bool {
	switch strings.ToUpper(s) {
	case "PLAIN", "LOGIN", "XOAUTH2", "OAUTHBEARER", "CRAM-MD5":
		return true
	}
	return false
}

var base64Line = regexp.MustCompile(`^[A-Za-z0-9+/=_-]*$`)

// isBase64Line: a line of only base64 characters (the continuation of an AUTH exchange; "+" is the
// server's request for it and is harmless).
func isBase64Line(s string) bool { return base64Line.MatchString(s) }
