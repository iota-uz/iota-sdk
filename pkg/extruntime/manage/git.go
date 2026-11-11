package manage

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Reasons of the errors of a Git import. They are the stable part of the
// answer: the output of the git process is never returned to the caller (it
// can name internal hosts, paths and refs); it is logged on the server.
const (
	ReasonGitDestinationDenied serrors.Reason = "git_destination_denied"
	ReasonGitFetchFailed       serrors.Reason = "git_fetch_failed"
	ReasonGitCommitNotFound    serrors.Reason = "git_commit_not_found"
	ReasonGitTooLarge          serrors.Reason = "git_too_large"
	ReasonGitUnexpected        serrors.Reason = "git_unexpected_output"
)

func gitError(code serrors.Code, reason serrors.Reason, msg string) *serrors.Error {
	return serrors.New(code, msg).WithReason(reason)
}

// GitConfig bounds repository imports.
type GitConfig struct {
	// Binary is the git executable. Default "git".
	Binary string
	// AllowedSchemes lists the transports an import may use: "https", "ssh",
	// "file" (a local path). Default https and ssh. "file" lets a caller read
	// any repository on the server, so enable it only for trusted setups.
	AllowedSchemes []string
	// AllowedHosts, when set, restricts https and ssh imports to these host
	// names: an exact name ("git.example.com") or "*.example.com" for any
	// subdomain. Empty: any host whose addresses pass the network check
	// below. Set it in production.
	AllowedHosts []string
	// AllowPrivateNetworks lets https and ssh imports reach loopback,
	// link-local (cloud metadata), private, shared and reserved addresses.
	// Off by default: a caller of the import API must not be able to make the
	// server fetch from its own network.
	AllowPrivateNetworks bool
	// Timeout bounds one import. Default 2 minutes.
	Timeout time.Duration
	// MaxFetchBytes bounds what one import may download into its scratch
	// repository. Default artifact.MaxTreeBytes plus one file.
	MaxFetchBytes int64
	// MaxFileBytes and MaxTreeBytes bound the files of the imported tree.
	// Defaults artifact.MaxFileBytes and artifact.MaxTreeBytes.
	MaxFileBytes, MaxTreeBytes int64
	// Lookup resolves a host name for the network check. Default: the
	// system resolver.
	Lookup func(ctx context.Context, host string) ([]netip.Addr, error)

	log *slog.Logger
}

func (c GitConfig) withDefaults() GitConfig {
	if c.Binary == "" {
		c.Binary = "git"
	}
	if len(c.AllowedSchemes) == 0 {
		c.AllowedSchemes = []string{"https", "ssh"}
	}
	if c.Timeout <= 0 {
		c.Timeout = 2 * time.Minute
	}
	if c.MaxFileBytes <= 0 {
		c.MaxFileBytes = artifact.MaxFileBytes
	}
	if c.MaxTreeBytes <= 0 {
		c.MaxTreeBytes = artifact.MaxTreeBytes
	}
	if c.MaxFetchBytes <= 0 {
		c.MaxFetchBytes = c.MaxTreeBytes + c.MaxFileBytes
	}
	if c.Lookup == nil {
		c.Lookup = func(ctx context.Context, host string) ([]netip.Addr, error) {
			return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		}
	}
	if c.log == nil {
		c.log = slog.Default()
	}
	return c
}

// remoteHelper matches git's "<helper>::<address>" form (ext::, fd::, ...),
// which runs a program; IPv6 literals ("[::1]") are not it.
var remoteHelper = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*::`)

var commitPattern = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// gitLocation is a classified repository location.
type gitLocation struct {
	scheme   string
	redacted string // safe to store and show: no credentials
	host     string // lower-case, no brackets, no trailing dot; "" for file
	port     string // "" when the URL names none
}

// locate classifies a repository location and applies the scheme and host
// allow-lists (no network access).
func (c GitConfig) locate(repo string) (gitLocation, error) {
	bad := func(msg string) (gitLocation, error) {
		return gitLocation{}, serrors.New(serrors.Invalid, "repository URL: "+msg)
	}
	if repo == "" || strings.HasPrefix(repo, "-") || remoteHelper.MatchString(repo) || strings.ContainsAny(repo, "\x00\n\r") {
		return bad("not allowed")
	}
	var loc gitLocation
	switch {
	case strings.Contains(repo, "://"):
		u, perr := url.Parse(repo)
		if perr != nil || u.Host == "" && u.Scheme != "file" {
			return bad("cannot be parsed")
		}
		loc.scheme = strings.ToLower(u.Scheme)
		if loc.scheme == "git+ssh" || loc.scheme == "ssh+git" {
			loc.scheme = "ssh"
		}
		loc.host, loc.port = normalizeHost(u.Hostname()), u.Port()
		u.User = nil
		loc.redacted = u.String()
	case strings.HasPrefix(repo, "/"):
		loc.scheme, loc.redacted = "file", repo
	case strings.Contains(repo, "@") && strings.Contains(repo, ":"):
		// scp-like user@host:path
		loc.scheme = "ssh"
		rest := repo[strings.Index(repo, "@")+1:]
		loc.redacted = rest
		loc.host = normalizeHost(strings.Trim(rest[:strings.Index(rest, ":")], "[]"))
	default:
		return bad("must be an https, ssh or absolute path location")
	}
	allowed := false
	for _, a := range c.AllowedSchemes {
		if a == loc.scheme {
			allowed = true
		}
	}
	if !allowed {
		return bad("scheme " + loc.scheme + " is not allowed")
	}
	if loc.scheme != "file" {
		if loc.host == "" {
			return bad("has no host")
		}
		if len(c.AllowedHosts) > 0 && !hostAllowed(c.AllowedHosts, loc.host) {
			return gitLocation{}, gitError(serrors.Invalid, ReasonGitDestinationDenied, "repository host is not allowed")
		}
	}
	return loc, nil
}

func normalizeHost(h string) string { return strings.TrimSuffix(strings.ToLower(h), ".") }

func hostAllowed(allowed []string, host string) bool {
	for _, a := range allowed {
		a = normalizeHost(strings.TrimSpace(a))
		if a == "" {
			continue
		}
		if suffix, ok := strings.CutPrefix(a, "*."); ok {
			if strings.HasSuffix(host, "."+suffix) {
				return true
			}
			continue
		}
		if a == host {
			return true
		}
	}
	return false
}

// blockedPrefixes are the non-public ranges netip does not classify itself.
var blockedPrefixes = func() []netip.Prefix {
	var out []netip.Prefix
	for _, p := range []string{
		"0.0.0.0/8",     // "this" network
		"100.64.0.0/10", // carrier-grade NAT
		"192.0.0.0/24",  // IETF protocol assignments
		"198.18.0.0/15", // benchmarking
		"240.0.0.0/4",   // reserved
		"64:ff9b::/96",  // NAT64
		"2002::/16",     // 6to4
		"fec0::/10",     // site-local (deprecated)
		"2001:db8::/32", // documentation
		"100::/64",      // discard
		"2001::/32",     // Teredo
	} {
		out = append(out, netip.MustParsePrefix(p))
	}
	return out
}()

func nonPublic(a netip.Addr) bool {
	a = a.Unmap()
	if a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() || a.IsMulticast() || a.IsUnspecified() || a.IsInterfaceLocalMulticast() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// destination resolves the host of an https or ssh location and checks every
// address; it returns the addresses the transport is pinned to, so the
// connection goes to what was checked, not to a second answer of the resolver.
func (c GitConfig) destination(ctx context.Context, loc gitLocation) ([]netip.Addr, error) {
	if loc.scheme == "file" {
		return nil, nil
	}
	denied := func() error {
		return gitError(serrors.Invalid, ReasonGitDestinationDenied, "repository host is not allowed")
	}
	var addrs []netip.Addr
	if a, err := netip.ParseAddr(loc.host); err == nil {
		addrs = []netip.Addr{a}
	} else {
		resolved, err := c.Lookup(ctx, loc.host)
		if err != nil || len(resolved) == 0 {
			c.log.Warn("extruntime git: host does not resolve", "host", loc.host, "error", err)
			return nil, gitError(serrors.Unavailable, ReasonGitFetchFailed, "cannot fetch the repository")
		}
		addrs = resolved
	}
	if !c.AllowPrivateNetworks {
		for _, a := range addrs {
			if nonPublic(a) {
				c.log.Warn("extruntime git: destination in a non-public range refused", "host", loc.host, "address", a.String())
				return nil, denied()
			}
		}
	}
	for i := range addrs {
		addrs[i] = addrs[i].Unmap()
	}
	return addrs, nil
}

// limitedBuffer keeps at most max bytes and reports whether more arrived.
type limitedBuffer struct {
	buf      bytes.Buffer
	max      int
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.max - b.buf.Len(); len(p) <= room {
		b.buf.Write(p)
	} else {
		if room > 0 {
			b.buf.Write(p[:room])
		}
		b.overflow = true
	}
	return len(p), nil
}

const stderrKept = 4 << 10

// gitJob prepares the git process of one import.
type gitJob struct {
	c    GitConfig
	dir  string
	loc  gitLocation
	pins []netip.Addr
}

func (j gitJob) command(ctx context.Context, args ...string) *exec.Cmd {
	global := []string{"-c", "protocol.version=2", "-c", "http.followRedirects=false", "-c", "core.fsmonitor=false"}
	sshCommand := "ssh -o BatchMode=yes"
	if len(j.pins) > 0 {
		list := make([]string, len(j.pins))
		for i, a := range j.pins {
			list[i] = a.String()
		}
		switch j.loc.scheme {
		case "https":
			port := j.loc.port
			if port == "" {
				port = "443"
			}
			// Connect to the addresses that were checked.
			global = append(global, "-c", "http.curloptResolve="+j.loc.host+":"+port+":"+strings.Join(list, ","))
		case "ssh":
			sshCommand += " -o HostName=" + list[0] + " -o HostKeyAlias=" + j.loc.host
		}
	}
	cmd := exec.CommandContext(ctx, j.c.Binary, append(global, args...)...)
	cmd.Dir = j.dir
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + j.dir,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_ALLOW_PROTOCOL=" + strings.Join(j.c.AllowedSchemes, ":"),
		"GIT_SSH_COMMAND=" + sshCommand,
	}
	cmd.WaitDelay = 5 * time.Second
	return cmd
}

// fail logs what git said on the server and returns the stable error.
func (j gitJob) fail(err error, stderr *limitedBuffer, args []string, code serrors.Code, reason serrors.Reason, msg string) error {
	j.c.log.Warn("extruntime git: command failed", "command", args[0], "error", err, "stderr", strings.TrimSpace(stderr.buf.String()))
	return gitError(code, reason, msg)
}

// run executes a git command with the output capped; the caller maps failure.
func (j gitJob) run(ctx context.Context, maxOut int, args ...string) ([]byte, *limitedBuffer, error) {
	cmd := j.command(ctx, args...)
	stdout, stderr := &limitedBuffer{max: maxOut}, &limitedBuffer{max: stderrKept}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		return nil, stderr, err
	}
	if stdout.overflow {
		return nil, stderr, errors.New("output too large")
	}
	return stdout.buf.Bytes(), stderr, nil
}

// fetch downloads into the scratch repository and stops the process as soon
// as the repository grows beyond MaxFetchBytes.
func (j gitJob) fetch(ctx context.Context, args ...string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := j.command(ctx, append([]string{"fetch", "-q", "--no-tags", "--no-recurse-submodules"}, args...)...)
	stderr := &limitedBuffer{max: stderrKept}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	var exceeded atomic.Bool
	done := make(chan struct{})
	watched := make(chan struct{})
	go func() {
		defer close(watched)
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				if dirSize(j.dir) > j.c.MaxFetchBytes {
					exceeded.Store(true)
					cancel()
					return
				}
			}
		}
	}()
	err := cmd.Wait()
	close(done)
	<-watched
	switch {
	case exceeded.Load() || dirSize(j.dir) > j.c.MaxFetchBytes:
		return gitError(serrors.Invalid, ReasonGitTooLarge, "the repository is too large to import")
	case err != nil:
		j.c.log.Warn("extruntime git: fetch failed", "error", err, "stderr", strings.TrimSpace(stderr.buf.String()))
		return err
	}
	return nil
}

func dirSize(dir string) int64 {
	var n int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // a file removed by git while walking is not an error
		}
		if info, err := d.Info(); err == nil {
			n += info.Size()
		}
		return nil
	})
	return n
}

type treeEntry struct {
	sha  string
	path string
	size int64
}

// fetchTree fetches exactly one commit and returns its tree (symlinks and
// submodules are not part of a package tree and are skipped).
func (c GitConfig) fetchTree(ctx context.Context, repo, commit, subdir string) (build.Tree, error) {
	const op = serrors.Op("extruntime.manage.fetchTree")
	if !commitPattern.MatchString(commit) {
		return nil, serrors.New(serrors.Invalid, "commit must be a full lowercase object id")
	}
	loc, err := c.locate(repo)
	if err != nil {
		return nil, err
	}
	prefix := ""
	if subdir != "" {
		clean, err := artifact.CleanPath(subdir)
		if err != nil {
			return nil, err
		}
		prefix = clean + "/"
	}
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	pins, err := c.destination(ctx, loc)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "extruntime-git-")
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	j := gitJob{c: c, dir: dir, loc: loc, pins: pins}
	unavailable := func() error {
		return gitError(serrors.Unavailable, ReasonGitFetchFailed, "cannot fetch the repository").WithOp(op)
	}

	if _, stderr, err := j.run(ctx, 1<<10, "init", "-q", "--bare", "."); err != nil {
		return nil, j.fail(err, stderr, []string{"init"}, serrors.Internal, ReasonGitUnexpected, "cannot prepare the import")
	}
	if err := j.fetch(ctx, "--depth=1", "--", repo, commit); err != nil {
		var se *serrors.Error
		if errors.As(err, &se) {
			return nil, se
		}
		// A server that refuses to serve a commit it does not advertise: ask
		// for the tips of its branches only (no history, no tags), still
		// within the size limit, and require the commit among them.
		if err2 := j.fetch(ctx, "--depth=1", "--", repo, "+refs/heads/*:refs/remotes/import/*"); err2 != nil {
			if errors.As(err2, &se) {
				return nil, se
			}
			return nil, unavailable()
		}
	}
	if _, stderr, err := j.run(ctx, 1<<10, "cat-file", "-e", commit+"^{commit}"); err != nil {
		c.log.Info("extruntime git: commit not found", "stderr", strings.TrimSpace(stderr.buf.String()))
		return nil, gitError(serrors.NotFound, ReasonGitCommitNotFound, "the commit is not in the repository").WithOp(op)
	}
	listing, stderr, err := j.run(ctx, 32<<20, "ls-tree", "-r", "-z", "-l", "--full-tree", commit)
	if err != nil {
		return nil, j.fail(err, stderr, []string{"ls-tree"}, serrors.Internal, ReasonGitUnexpected, "cannot read the repository tree")
	}
	entries, err := c.selectEntries(listing, prefix, subdir)
	if err != nil {
		return nil, err
	}
	return j.readBlobs(ctx, entries)
}

// selectEntries picks the regular files under prefix from an `ls-tree -l`
// listing and checks the limits on their declared sizes before any content is
// read.
func (c GitConfig) selectEntries(listing []byte, prefix, subdir string) ([]treeEntry, error) {
	const op = serrors.Op("extruntime.manage.fetchTree")
	var entries []treeEntry
	var total int64
	for _, rec := range bytes.Split(listing, []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		meta, p, ok := bytes.Cut(rec, []byte{'\t'})
		if !ok {
			return nil, gitError(serrors.Internal, ReasonGitUnexpected, "unexpected repository listing").WithOp(op)
		}
		fields := strings.Fields(string(meta))
		if len(fields) != 4 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
			continue // symlinks (120000), submodules (160000)
		}
		name := string(p)
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		size, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil || size < 0 {
			return nil, gitError(serrors.Internal, ReasonGitUnexpected, "unexpected repository listing").WithOp(op)
		}
		rel := strings.TrimPrefix(name, prefix)
		if _, err := artifact.CleanPath(path.Clean(rel)); err != nil {
			continue // .git or node_modules content committed by mistake
		}
		if size > c.MaxFileBytes {
			return nil, gitError(serrors.Invalid, ReasonGitTooLarge, "a file of the repository is too large").WithOp(op)
		}
		total += size
		if total > c.MaxTreeBytes {
			return nil, gitError(serrors.Invalid, ReasonGitTooLarge, "the repository tree is too large").WithOp(op)
		}
		entries = append(entries, treeEntry{sha: fields[2], path: rel, size: size})
		if len(entries) > artifact.MaxFiles {
			return nil, gitError(serrors.Invalid, ReasonGitTooLarge, "the repository tree has too many files").WithOp(op)
		}
	}
	if len(entries) == 0 {
		return nil, serrors.New(serrors.NotFound, "the commit has no files under "+strconv.Quote(subdir)).WithOp(op)
	}
	return entries, nil
}

// readBlobs streams the blobs through `cat-file --batch`: a blob is read
// only after its header says it is the size the tree listing declared, and
// never more than that is allocated.
func (j gitJob) readBlobs(ctx context.Context, entries []treeEntry) (build.Tree, error) {
	const op = serrors.Op("extruntime.manage.fetchTree")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := j.command(ctx, "cat-file", "--batch")
	stderr := &limitedBuffer{max: stderrKept}
	cmd.Stderr = stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	go func() {
		defer func() { _ = in.Close() }()
		w := bufio.NewWriter(in)
		for _, e := range entries {
			if _, err := w.WriteString(e.sha + "\n"); err != nil {
				return
			}
		}
		_ = w.Flush()
	}()
	unexpected := func(err error) error {
		cancel()
		_ = cmd.Wait()
		return j.fail(err, stderr, []string{"cat-file"}, serrors.Internal, ReasonGitUnexpected, "unexpected repository content")
	}
	r := bufio.NewReaderSize(out, 64<<10)
	tree := build.Tree{}
	for _, e := range entries {
		head, err := r.ReadString('\n')
		if err != nil {
			return nil, unexpected(err)
		}
		parts := strings.Fields(head)
		if len(parts) != 3 || parts[1] != "blob" {
			return nil, unexpected(fmt.Errorf("header %q", head))
		}
		size, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || size != e.size {
			return nil, unexpected(fmt.Errorf("size %q for a listed size of %d", parts[2], e.size))
		}
		body := make([]byte, size)
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, unexpected(err)
		}
		if _, err := r.ReadByte(); err != nil { // trailing newline
			return nil, unexpected(err)
		}
		clean, err := artifact.CleanPath(path.Clean(e.path))
		if err != nil {
			continue
		}
		tree[clean] = body
	}
	if err := cmd.Wait(); err != nil {
		return nil, j.fail(err, stderr, []string{"cat-file"}, serrors.Internal, ReasonGitUnexpected, "unexpected repository content")
	}
	return tree, nil
}

// ImportGit imports one explicit commit of a repository as the package
// draft. The commit must be a full object id; a branch or tag name is never
// resolved, so the import is reproducible. subdir selects a package inside a
// monorepo ("" = repository root).
func (s *Service) ImportGit(ctx context.Context, actor spi.Subject, repoURL, commit, subdir string) (Package, error) {
	e := auditEntry{Action: ActionImportGit, Detail: map[string]any{"commit": commit, "subdir": subdir}}
	loc, serr := s.git.locate(repoURL)
	e.Detail["repoUrl"] = loc.redacted
	if !actor.Has(PermAuthor) {
		return Package{}, s.deny(ctx, actor, PermAuthor, e)
	}
	if serr != nil {
		s.failed(ctx, actor, e, serr)
		return Package{}, serr
	}
	tree, err := s.git.fetchTree(ctx, repoURL, commit, subdir)
	if err != nil {
		s.failed(ctx, actor, e, err)
		return Package{}, err
	}
	return s.importTree(ctx, actor, ActionImportGit, tree, ImportMeta{Provenance: ProvenanceGit, RepoURL: loc.redacted, RepoCommit: commit})
}

func withLog(c GitConfig, l *slog.Logger) GitConfig {
	if l != nil {
		c.log = l
	}
	return c
}
