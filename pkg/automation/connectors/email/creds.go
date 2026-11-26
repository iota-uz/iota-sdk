package email

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

// account is how a mailbox is reached, from a connection's credentials.
type account struct {
	preset   string
	host     string
	port     int
	username string
	password string
	// from is the sender of an smtp account; mailbox the folder of an imap one.
	fromAddress, fromName string
	mailbox               string
	loginID               string
	// security is how the connection is secured: tls (from the first byte), starttls, or auto (by port).
	security string
}

// oauth reports whether the account signs in with a token (gmail, m365).
func (a account) oauth() bool { return a.preset == PresetGmail || a.preset == PresetM365 }

// implicitTLS: with security auto, 465 (SMTP) and 993 (IMAP) are TLS from the first byte and every other
// port must offer STARTTLS; a login never crosses the network in the clear either way.
func (a account) implicitTLS(imap bool) bool {
	switch a.security {
	case "tls":
		return true
	case "starttls":
		return false
	}
	if imap {
		return a.port == 993
	}
	return a.port == 465
}

func (a account) addr() string { return net.JoinHostPort(a.host, strconv.Itoa(a.port)) }

func itoa(n int) string { return strconv.Itoa(n) }

func (h *hub) parseAccount(c connectors.Credentials, imap bool) (account, error) {
	a := account{
		preset:      strings.TrimSpace(c["preset"]),
		host:        strings.TrimSpace(c["host"]),
		username:    strings.TrimSpace(c["username"]),
		password:    c["password"],
		fromAddress: strings.TrimSpace(c["from_address"]),
		fromName:    strings.TrimSpace(c["from_name"]),
		mailbox:     strings.TrimSpace(c["mailbox"]),
		loginID:     c["login_id"],
		security:    strings.TrimSpace(c["security"]),
	}
	if a.security != "" && a.security != "auto" && a.security != "tls" && a.security != "starttls" {
		return account{}, errors.New("email: security is auto, tls or starttls (status 400)")
	}
	if a.preset == "" {
		a.preset = PresetCustom
	}
	if p := strings.TrimSpace(c["port"]); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return account{}, errors.New("email: the port is a number from 1 to 65535 (status 400)")
		}
		a.port = n
	}
	switch a.preset {
	case PresetGmail:
		a.host, a.port, a.security = h.server(PresetGmail, imap, "imap.gmail.com", 993, "smtp.gmail.com", 587)
	case PresetM365:
		a.host, a.port, a.security = h.server(PresetM365, imap, "outlook.office365.com", 993, "smtp.office365.com", 587)
	case PresetCustom:
		if a.host == "" {
			return account{}, errors.New("email: the server is not set (status 400)")
		}
		if a.port == 0 {
			a.port = map[bool]int{true: 993, false: 587}[imap]
		}
	default:
		return account{}, errors.New("email: the preset is custom, gmail or m365 (status 400)")
	}
	if a.mailbox == "" {
		a.mailbox = "INBOX"
	}
	if a.oauth() && a.username == "" {
		// Credentials a sign-in has just produced (the connector's Test, before they are saved) name the
		// mailbox only by their identity token.
		a.username, _ = addressOfIDToken(c[connectors.OAuth2IDToken])
	}
	if a.oauth() && a.username == "" {
		return account{}, errors.New("email: the sign-in does not name its mailbox; sign in again (status 401)")
	}
	if a.fromAddress == "" && a.oauth() {
		a.fromAddress = a.username
	}
	return a, nil
}

// server is the host and port of a preset's server, and how it is secured: the provider's, unless the
// application points the preset elsewhere (Config.Providers).
func (h *hub) server(preset string, imap bool, ih string, ip int, sh string, sp int) (string, int, string) {
	host, port := pick(imap, ih, ip, sh, sp)
	p := h.cfg.Providers[preset]
	// A host and a port are overridden each on its own: a host given alone keeps the provider's port.
	pickHost, pickPort := p.SMTPHost, p.SMTPPort
	if imap {
		pickHost, pickPort = p.IMAPHost, p.IMAPPort
	}
	if pickHost != "" {
		host = pickHost
	}
	if pickPort != 0 {
		port = pickPort
	}
	return host, port, p.Security
}

func pick(imap bool, ih string, ip int, sh string, sp int) (string, int) {
	if imap {
		return ih, ip
	}
	return sh, sp
}

func (h *hub) parseSMTP(c connectors.Credentials) (account, error) {
	a, err := h.parseAccount(c, false)
	if err != nil {
		return a, err
	}
	if a.fromAddress == "" || mailboxAddress(a.fromAddress) == "" {
		return account{}, errors.New("email: the sender address is not an email address (status 400)")
	}
	return a, nil
}

func (h *hub) parseIMAP(c connectors.Credentials) (account, error) {
	a, err := h.parseAccount(c, true)
	if err != nil {
		return a, err
	}
	if a.username == "" || (!a.oauth() && a.password == "") {
		return account{}, errors.New("email: a login and a password are required (status 400)")
	}
	return a, nil
}

// tokenOf is the access token of credentials a sign-in just produced (Test, before the connection exists).
func tokenOf(c connectors.Credentials) (string, error) {
	if t := c[connectors.OAuth2AccessToken]; t != "" {
		return t, nil
	}
	return "", errors.New("email: no access token (status 401)")
}

// mailboxAddress is the bare address of "Name <a@b>" or "a@b"; empty when it is not an address.
func mailboxAddress(s string) string {
	a, err := mail.ParseAddress(strings.TrimSpace(s))
	if err != nil {
		return ""
	}
	return a.Address
}

// identityOf is the public identity of a mailbox: its address, when one of the candidates is.
func identityOf(candidates ...string) connectors.Identity {
	for _, c := range candidates {
		if addr := mailboxAddress(c); addr != "" {
			return connectors.Identity{Display: addr, External: strings.ToLower(addr), Kind: connectors.IdentityEmail}
		}
	}
	return connectors.Identity{}
}

// addressOfIDToken is the mailbox an OpenID Connect token is for: its email claim, else the preferred
// username (Microsoft's). The token came from the provider's token endpoint over TLS, so its signature is
// not checked (OpenID Connect Core 3.1.3.7).
func addressOfIDToken(tok string) (string, error) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return "", errNoMailbox
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return "", errNoMailbox
	}
	var claims struct {
		Email             string `json:"email"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return "", errNoMailbox
	}
	for _, c := range []string{claims.Email, claims.PreferredUsername} {
		if a := mailboxAddress(c); a != "" {
			return a, nil
		}
	}
	return "", errNoMailbox
}

// refused is the error of a server that does not accept the credentials: unauthorized, so a poll
// stops (needs_reauth) and a node fails with a code a graph can branch on.
func refused(proto string, status int) error {
	return &catalog.CallError{Code: catalog.CallUnauthorized, Message: fmt.Sprintf("%s authentication failed (status %d)", proto, status)}
}

// uuidOf parses a connection id; "platform:smtp" and the like are not ones.
func uuidOf(s string) (uuid.UUID, error) { return uuid.Parse(s) }
