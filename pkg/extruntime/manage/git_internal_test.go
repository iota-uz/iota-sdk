package manage

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	req "github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func addrs(ss ...string) func(context.Context, string) ([]netip.Addr, error) {
	return func(context.Context, string) ([]netip.Addr, error) {
		out := make([]netip.Addr, len(ss))
		for i, s := range ss {
			out[i] = netip.MustParseAddr(s)
		}
		return out, nil
	}
}

func TestGitDestinationPolicy(t *testing.T) {
	ctx := context.Background()
	check := func(c GitConfig, repo string) error {
		c = c.withDefaults()
		loc, err := c.locate(repo)
		if err != nil {
			return err
		}
		_, err = c.destination(ctx, loc)
		return err
	}
	public := GitConfig{Lookup: addrs("93.184.216.34")}

	req.NoError(t, check(public, "https://git.example.com/org/repo.git"))
	req.NoError(t, check(public, "git@git.example.com:org/repo.git"))

	t.Run("non-public addresses are refused", func(t *testing.T) {
		for _, ip := range []string{
			"127.0.0.1", "127.1.2.3", "::1", "10.0.0.5", "172.16.9.9", "192.168.1.1", "169.254.169.254", "fe80::1",
			"100.64.0.1", "0.0.0.0", "0.1.2.3", "192.0.0.8", "198.19.0.1", "240.0.0.1", "224.0.0.1", "fc00::1", "::ffff:127.0.0.1",
			"::ffff:169.254.169.254", "64:ff9b::7f00:1", "fec0::1",
		} {
			err := check(GitConfig{Lookup: addrs(ip)}, "https://git.example.com/x.git")
			req.Equal(t, ReasonGitDestinationDenied, serrors.ReasonOf(err), ip)
			req.Equal(t, serrors.Invalid, serrors.CodeOf(err), ip)
		}
	})
	t.Run("one bad address among good ones refuses the host", func(t *testing.T) {
		err := check(GitConfig{Lookup: addrs("93.184.216.34", "10.0.0.5")}, "https://git.example.com/x.git")
		req.Equal(t, ReasonGitDestinationDenied, serrors.ReasonOf(err))
	})
	t.Run("literal addresses and localhost are checked without a lookup", func(t *testing.T) {
		never := GitConfig{Lookup: func(context.Context, string) ([]netip.Addr, error) { return nil, errors.New("must not resolve") }}
		for _, repo := range []string{
			"https://127.0.0.1/x.git", "https://[::1]/x.git", "https://169.254.169.254/latest", "ssh://git@10.1.2.3/x.git",
			"git@192.168.0.9:org/x.git", "https://[::ffff:10.0.0.1]/x.git",
		} {
			req.Equal(t, ReasonGitDestinationDenied, serrors.ReasonOf(check(never, repo)), "%s: %v", repo, check(never, repo))
		}
		err := check(GitConfig{Lookup: addrs("127.0.0.1")}, "https://localhost:8443/x.git")
		req.Equal(t, ReasonGitDestinationDenied, serrors.ReasonOf(err), "localhost resolves to loopback")
	})
	t.Run("private networks can be allowed explicitly", func(t *testing.T) {
		req.NoError(t, check(GitConfig{Lookup: addrs("10.0.0.5"), AllowPrivateNetworks: true}, "https://git.corp/x.git"))
		req.NoError(t, check(GitConfig{AllowPrivateNetworks: true}, "https://127.0.0.1/x.git"))
	})
	t.Run("an unresolvable host is an unavailable repository, not a detail", func(t *testing.T) {
		err := check(GitConfig{Lookup: func(context.Context, string) ([]netip.Addr, error) {
			return nil, errors.New("dial 10.9.9.9: secret detail")
		}}, "https://git.example.com/x.git")
		req.Equal(t, serrors.Unavailable, serrors.CodeOf(err))
		req.NotContains(t, err.Error(), "secret detail")
	})
	t.Run("the host allow-list", func(t *testing.T) {
		c := GitConfig{Lookup: addrs("93.184.216.34"), AllowedHosts: []string{"git.example.com", "*.corp.example.com"}}
		for _, ok := range []string{"https://git.example.com/x.git", "https://GIT.example.com./x.git", "https://a.corp.example.com/x.git", "git@x.y.corp.example.com:o/r.git"} {
			req.NoError(t, check(c, ok), ok)
		}
		for _, no := range []string{"https://example.com/x.git", "https://corp.example.com/x.git", "https://evilgit.example.com/x.git", "https://git.example.com.evil.net/x.git", "https://user@evil.net/x.git"} {
			req.Equal(t, ReasonGitDestinationDenied, serrors.ReasonOf(check(c, no)), no)
		}
	})
	t.Run("git helper transports and option-looking locations are refused", func(t *testing.T) {
		for _, repo := range []string{"ext::sh -c touch /tmp/x", "fd::3", "--upload-pack=x", "-oProxyCommand=x", "https://x\ny/"} {
			req.Equal(t, serrors.Invalid, serrors.CodeOf(check(GitConfig{AllowedSchemes: []string{"https", "ssh", "file"}}, repo)), repo)
		}
	})
	t.Run("a url cannot smuggle another host through its user info", func(t *testing.T) {
		c := GitConfig{Lookup: addrs("93.184.216.34"), AllowedHosts: []string{"git.example.com"}}
		req.Equal(t, ReasonGitDestinationDenied, serrors.ReasonOf(check(c, "https://git.example.com@evil.net/x.git")))
	})
}

// repo makes a bare repository with the given files and returns it and its commit.
func bareOf(t *testing.T, files map[string][]byte) (string, string) {
	t.Helper()
	run := func(dir string, args ...string) string {
		cmd := exec.Command("git", append([]string{"-c", "user.name=T", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "HOME="+t.TempDir(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		req.NoError(t, err, string(out))
		return strings.TrimSpace(string(out))
	}
	work := t.TempDir()
	run(work, "init", "-q")
	for p, c := range files {
		full := filepath.Join(work, filepath.FromSlash(p))
		req.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		req.NoError(t, os.WriteFile(full, c, 0o644))
	}
	run(work, "add", "-A")
	run(work, "commit", "-q", "-m", "x")
	sha := run(work, "rev-parse", "HEAD")
	bare := filepath.Join(t.TempDir(), "r.git")
	run(work, "clone", "-q", "--bare", work, bare)
	return bare, sha
}

func fileConfig() GitConfig {
	return GitConfig{AllowedSchemes: []string{"file"}, Timeout: time.Minute}.withDefaults()
}

func TestGitImportLimitsAreCheckedBeforeContentIsRead(t *testing.T) {
	ctx := context.Background()
	bare, sha := bareOf(t, map[string][]byte{"a.txt": []byte("small"), "big.bin": make([]byte, 3<<10)})

	c := fileConfig()
	tree, err := c.fetchTree(ctx, bare, sha, "")
	req.NoError(t, err)
	req.Equal(t, []byte("small"), tree["a.txt"])
	req.Len(t, tree["big.bin"], 3<<10)

	c.MaxFileBytes = 1 << 10
	_, err = c.fetchTree(ctx, bare, sha, "")
	req.Equal(t, ReasonGitTooLarge, serrors.ReasonOf(err), "one file over the per-file limit")

	c = fileConfig()
	c.MaxTreeBytes = 2 << 10
	_, err = c.fetchTree(ctx, bare, sha, "")
	req.Equal(t, ReasonGitTooLarge, serrors.ReasonOf(err), "the tree over the total limit")

	c = fileConfig()
	c.MaxFetchBytes = 1 << 10
	_, err = c.fetchTree(ctx, bare, sha, "")
	req.Equal(t, ReasonGitTooLarge, serrors.ReasonOf(err), "the download over the fetch limit")
}

func TestGitImportErrorsNeverCarryGitOutput(t *testing.T) {
	ctx := context.Background()
	c := fileConfig()
	secret := filepath.Join(t.TempDir(), "internal-secret-repo-name.git")

	_, err := c.fetchTree(ctx, secret, strings.Repeat("a", 40), "")
	req.Error(t, err)
	req.Equal(t, ReasonGitFetchFailed, serrors.ReasonOf(err))
	req.Equal(t, serrors.Unavailable, serrors.CodeOf(err))
	req.NotContains(t, err.Error(), "internal-secret-repo-name")
	req.NotContains(t, err.Error(), "fatal")

	bare, _ := bareOf(t, map[string][]byte{"a": []byte("x")})
	_, err = c.fetchTree(ctx, bare, strings.Repeat("0", 40), "")
	req.Equal(t, ReasonGitCommitNotFound, serrors.ReasonOf(err))
	req.Equal(t, serrors.NotFound, serrors.CodeOf(err))
	req.NotContains(t, err.Error(), bare)
}

func TestGitImportFetchesOnlyTheRequestedCommit(t *testing.T) {
	ctx := context.Background()
	bare, first := bareOf(t, map[string][]byte{"a.txt": []byte("one")})
	// A second commit and a branch in the same repository must not be needed.
	cmd := exec.Command("git", "-C", bare, "rev-list", "--count", first)
	out, err := cmd.Output()
	req.NoError(t, err)
	req.Equal(t, "1", strings.TrimSpace(string(out)))

	tree, err := fileConfig().fetchTree(ctx, bare, first, "")
	req.NoError(t, err)
	req.Equal(t, []byte("one"), tree["a.txt"])
}
