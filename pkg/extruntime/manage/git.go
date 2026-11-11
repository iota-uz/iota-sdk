package manage

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// GitConfig bounds repository imports.
type GitConfig struct {
	// Binary is the git executable. Default "git".
	Binary string
	// AllowedSchemes lists the transports an import may use: "https", "ssh",
	// "file" (a local path). Default https and ssh. "file" lets a caller read
	// any repository on the server, so enable it only for trusted setups.
	AllowedSchemes []string
	// Timeout bounds one import. Default 2 minutes.
	Timeout time.Duration
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
	return c
}

var commitPattern = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// scheme classifies a repository location and returns a form safe to store.
func (c GitConfig) scheme(repo string) (scheme, redacted string, err error) {
	bad := func(msg string) (string, string, error) {
		return "", "", serrors.New(serrors.Invalid, "repository URL: "+msg)
	}
	if repo == "" || strings.HasPrefix(repo, "-") || strings.Contains(repo, "::") || strings.ContainsAny(repo, "\x00\n\r") {
		return bad("not allowed")
	}
	switch {
	case strings.Contains(repo, "://"):
		u, perr := url.Parse(repo)
		if perr != nil || u.Host == "" && u.Scheme != "file" {
			return bad("cannot be parsed")
		}
		scheme = strings.ToLower(u.Scheme)
		if scheme == "git+ssh" || scheme == "ssh+git" {
			scheme = "ssh"
		}
		u.User = nil
		redacted = u.String()
	case strings.HasPrefix(repo, "/"):
		scheme, redacted = "file", repo
	case strings.Contains(repo, "@") && strings.Contains(repo, ":"):
		// scp-like user@host:path
		scheme = "ssh"
		redacted = repo[strings.Index(repo, "@")+1:]
	default:
		return bad("must be an https, ssh or absolute path location")
	}
	for _, a := range c.AllowedSchemes {
		if a == scheme {
			return scheme, redacted, nil
		}
	}
	return bad("scheme " + scheme + " is not allowed")
}

func (c GitConfig) run(ctx context.Context, dir string, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.Binary, args...)
	cmd.Dir = dir
	cmd.Stdin = stdin
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + dir,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_ALLOW_PROTOCOL=" + strings.Join(c.AllowedSchemes, ":"),
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes",
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 400 {
			msg = msg[:400]
		}
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, msg)
	}
	return stdout.Bytes(), nil
}

// fetchTree fetches exactly one commit and returns its tree (symlinks and
// submodules are not part of a package tree and are skipped).
func (c GitConfig) fetchTree(ctx context.Context, repo, commit, subdir string) (build.Tree, error) {
	const op = serrors.Op("extruntime.manage.fetchTree")
	if !commitPattern.MatchString(commit) {
		return nil, serrors.New(serrors.Invalid, "commit must be a full lowercase object id")
	}
	if _, _, err := c.scheme(repo); err != nil {
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
	dir, err := os.MkdirTemp("", "extruntime-git-")
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	if _, err := c.run(ctx, dir, nil, "init", "-q", "--bare", "."); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	if _, err := c.run(ctx, dir, nil, "fetch", "-q", "--depth=1", "--no-tags", "--no-recurse-submodules", "--", repo, commit); err != nil {
		// Servers that refuse a fetch of an unadvertised object: take the
		// advertised refs and require the commit among them.
		if _, err2 := c.run(ctx, dir, nil, "fetch", "-q", "--no-tags", "--no-recurse-submodules", "--", repo,
			"+refs/heads/*:refs/remotes/import/*", "+refs/tags/*:refs/tags/*"); err2 != nil {
			return nil, serrors.New(serrors.Unavailable, "cannot fetch repository: "+err.Error()).WithOp(op)
		}
	}
	if _, err := c.run(ctx, dir, nil, "cat-file", "-e", commit+"^{commit}"); err != nil {
		return nil, serrors.New(serrors.NotFound, "commit "+commit+" is not in the repository").WithOp(op)
	}
	listing, err := c.run(ctx, dir, nil, "ls-tree", "-r", "-z", "--full-tree", commit)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	type entry struct{ sha, path string }
	var entries []entry
	for _, rec := range bytes.Split(listing, []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		meta, p, ok := bytes.Cut(rec, []byte{'\t'})
		if !ok {
			return nil, serrors.New(serrors.Internal, "unexpected git ls-tree output").WithOp(op)
		}
		fields := strings.Fields(string(meta))
		if len(fields) != 3 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
			continue // symlinks (120000), submodules (160000)
		}
		name := string(p)
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		entries = append(entries, entry{fields[2], strings.TrimPrefix(name, prefix)})
	}
	if len(entries) == 0 {
		return nil, serrors.New(serrors.NotFound, "the commit has no files under "+strconv.Quote(subdir)).WithOp(op)
	}
	if len(entries) > artifact.MaxFiles {
		return nil, serrors.New(serrors.Invalid, "the repository tree has too many files").WithOp(op)
	}
	var in bytes.Buffer
	for _, e := range entries {
		in.WriteString(e.sha + "\n")
	}
	out, err := c.run(ctx, dir, &in, "cat-file", "--batch")
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	r := bufio.NewReader(bytes.NewReader(out))
	tree := build.Tree{}
	var total int64
	for _, e := range entries {
		head, err := r.ReadString('\n')
		if err != nil {
			return nil, serrors.New(serrors.Internal, "unexpected git cat-file output").WithOp(op)
		}
		parts := strings.Fields(head)
		if len(parts) != 3 || parts[1] != "blob" {
			return nil, serrors.New(serrors.Internal, "unexpected git cat-file header").WithOp(op)
		}
		size, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || size < 0 || size > artifact.MaxFileBytes {
			return nil, serrors.New(serrors.Invalid, "file "+e.path+" is too large").WithOp(op)
		}
		body := make([]byte, size)
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		if _, err := r.ReadByte(); err != nil { // trailing newline
			return nil, serrors.Wrap(op, err)
		}
		clean, err := artifact.CleanPath(path.Clean(e.path))
		if err != nil {
			continue // .git or node_modules content committed by mistake
		}
		total += size
		if total > artifact.MaxTreeBytes {
			return nil, serrors.New(serrors.Invalid, "the repository tree is too large").WithOp(op)
		}
		tree[clean] = body
	}
	return tree, nil
}

// ImportGit imports one explicit commit of a repository as the package
// draft. The commit must be a full object id; a branch or tag name is never
// resolved, so the import is reproducible. subdir selects a package inside a
// monorepo ("" = repository root).
func (s *Service) ImportGit(ctx context.Context, actor spi.Subject, repoURL, commit, subdir string) (Package, error) {
	e := auditEntry{Action: ActionImportGit, Detail: map[string]any{"commit": commit, "subdir": subdir}}
	_, redacted, serr := s.git.scheme(repoURL)
	e.Detail["repoUrl"] = redacted
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
	return s.importTree(ctx, actor, ActionImportGit, tree, ImportMeta{Provenance: ProvenanceGit, RepoURL: redacted, RepoCommit: commit})
}
