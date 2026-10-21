//go:build dev

package agentsignin

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func testOptions(t *testing.T) Options {
	t.Helper()
	return Options{Enabled: true, Environment: "development", Origin: "http://localhost:3200", TenantID: uuid.New(), Directory: filepath.Join(t.TempDir(), "tickets")}
}

// Falsely green if requests consume different tokens instead of racing for the same grant.
func TestTicketSingleUseConcurrent(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	link, err := Issue(o, "42", "/crm")
	require.NoError(t, err)
	token := strings.TrimPrefix(strings.Split(link, "#")[1], "token=")
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := consume(o, token); err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, successes.Load())
}

// Falsely green if the expiry or origin is changed only in an unused copy of the ticket.
func TestTicketExpiryAndBinding(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"expired", "origin", "tenant"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			o := testOptions(t)
			link, err := Issue(o, "42", "/crm")
			require.NoError(t, err)
			u, err := url.Parse(link)
			require.NoError(t, err)
			token := strings.TrimPrefix(u.Fragment, "token=")
			dir, err := o.directory()
			require.NoError(t, err)
			path, err := ticketPath(dir, token)
			require.NoError(t, err)
			b, err := os.ReadFile(path)
			require.NoError(t, err)
			var grant ticket
			require.NoError(t, json.Unmarshal(b, &grant))
			switch scenario {
			case "expired":
				grant.Expires = time.Now().Add(-time.Second)
			case "origin":
				grant.Origin = "http://localhost:9999"
			case "tenant":
				grant.TenantID = uuid.New()
			}
			b, err = json.Marshal(grant)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, b, 0600))
			_, err = consume(o, token)
			require.ErrorIs(t, err, ErrTicket)
		})
	}
}

// Falsely green if unsafe redirects never reach Issue or the development gate is skipped.
func TestIssueRejectsUnsafeConfigurationAndRedirect(t *testing.T) {
	t.Parallel()
	for _, next := range []string{"https://example.com", "//example.com", "/\\example.com", "relative", "/\r\nLocation: x"} {
		_, err := Issue(testOptions(t), "42", next)
		require.Error(t, err)
	}
	for _, environment := range []string{"production", "staging", "preprod", ""} {
		o := testOptions(t)
		o.Environment = environment
		_, err := Issue(o, "42", "/")
		require.Error(t, err)
	}
	o := testOptions(t)
	o.Enabled = false
	_, err := Issue(o, "42", "/")
	require.Error(t, err)
	o = testOptions(t)
	o.Origin = "http://erp.eai.uz"
	_, err = Issue(o, "42", "/")
	require.Error(t, err)
}

// Falsely green if JSON output contains extra log lines that prevent an agent from parsing it.
func TestCommandOutputsUsableURL(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	cmd := NewCommand(func() (Options, error) { return o, nil })
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"sign-in", "--user", "42", "--output", "json"})
	require.NoError(t, cmd.Execute())
	var result struct {
		URL string `json:"url"`
	}
	require.NoError(t, json.Unmarshal(output.Bytes(), &result))
	u, err := url.Parse(result.URL)
	require.NoError(t, err)
	_, err = consume(o, strings.TrimPrefix(u.Fragment, "token="))
	require.NoError(t, err)
}
