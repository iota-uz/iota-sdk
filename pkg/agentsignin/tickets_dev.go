//go:build dev

package agentsignin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrTicket = errors.New("invalid, expired or consumed agent sign-in ticket")

type ticket struct {
	User     string
	Origin   string
	TenantID uuid.UUID
	Next     string
	Expires  time.Time
}

func privateDirectory(o Options) (string, error) {
	dir, err := o.directory()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return "", errors.New("agent sign-in directory must be private (0700), not a symlink")
	}
	return dir, nil
}

func ticketPath(dir, token string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(b) != 32 {
		return "", ErrTicket
	}
	digest := sha256.Sum256(b)
	return filepath.Join(dir, fmt.Sprintf("%x.json", digest)), nil
}

// Issue creates a one-minute, single-use capability bound to the origin and tenant.
func Issue(o Options, selector, next string) (string, error) {
	if err := o.Validate(); err != nil {
		return "", err
	}
	if strings.TrimSpace(selector) == "" {
		return "", errors.New("user ID or email is required")
	}
	u, err := url.Parse(next)
	if err != nil || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.ContainsAny(next, "\\\r\n") || u.IsAbs() || u.Host != "" {
		return "", errors.New("next must be an internal absolute path")
	}
	dir, err := privateDirectory(o)
	if err != nil {
		return "", err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	path, err := ticketPath(dir, token)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(ticket{User: selector, Origin: o.Origin, TenantID: o.TenantID, Next: next, Expires: time.Now().Add(time.Minute)})
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	_, writeErr := f.Write(payload)
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return o.Origin + Path + "#token=" + token, nil
}

func consume(o Options, token string) (ticket, error) {
	dir, err := privateDirectory(o)
	if err != nil {
		return ticket{}, err
	}
	path, err := ticketPath(dir, token)
	if err != nil {
		return ticket{}, err
	}
	// Atomic rename claims the capability across processes and concurrent requests.
	claimed := path + "." + uuid.NewString() + ".claimed"
	if err := os.Rename(path, claimed); err != nil {
		return ticket{}, ErrTicket
	}
	defer os.Remove(claimed)
	info, err := os.Lstat(claimed)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 4096 {
		return ticket{}, ErrTicket
	}
	b, err := os.ReadFile(claimed)
	if err != nil {
		return ticket{}, ErrTicket
	}
	var t ticket
	if json.Unmarshal(b, &t) != nil || t.Origin != o.Origin || t.TenantID != o.TenantID || !time.Now().Before(t.Expires) {
		return ticket{}, ErrTicket
	}
	return t, nil
}
