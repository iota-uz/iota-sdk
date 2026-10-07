// Package agentsignin provides a CLI-to-browser handoff for local development.
package agentsignin

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/agentsession"
)

const Path = "/dev/agent-sign-in"

type Options struct {
	Enabled     bool
	Environment string
	Origin      string
	TenantID    uuid.UUID
	Directory   string
}

func (o Options) Validate() error {
	if !o.Enabled || !agentsession.Allowed(o.Environment) {
		return errors.New("agent sign-in requires explicit enablement, a dev build and development environment")
	}
	u, err := url.Parse(o.Origin)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return errors.New("agent sign-in origin must be an HTTP loopback origin without a path")
	}
	if !loopback(u.Hostname()) || o.TenantID == uuid.Nil {
		return errors.New("agent sign-in requires a loopback origin and a tenant ID")
	}
	return nil
}

func loopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (o Options) directory() (string, error) {
	if o.Directory != "" {
		return filepath.Abs(o.Directory)
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(o.Origin + "|" + o.TenantID.String()))
	return filepath.Join(base, "iota-agent-sign-in", fmt.Sprintf("%x", digest)), nil
}
