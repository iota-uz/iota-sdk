package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (g GitHub) status(ctx context.Context, sha, state, description string) error {
	return g.API(ctx, "POST", "statuses/"+sha, map[string]string{
		"context": "sdk/release", "state": state, "description": description,
	}, nil)
}

func (g GitHub) ReconcileConsumers(ctx context.Context) error {
	var failures []error
	for page := 1; ; page++ {
		var prs []Pull
		if err := g.API(ctx, "GET", fmt.Sprintf("pulls?state=open&per_page=100&page=%d", page), nil, &prs); err != nil {
			return err
		}
		for _, pr := range prs {
			if pr.Head.Repo.FullName != g.Repo {
				continue
			}
			if err := g.reconcileConsumer(ctx, pr); err != nil {
				_ = g.status(ctx, pr.Head.SHA, "failure", "SDK dependency failed; inspect the dependency workflow")
				failures = append(failures, fmt.Errorf("consumer PR #%d: %w", pr.Number, err))
			}
		}
		if len(prs) < 100 {
			break
		}
	}
	return errors.Join(failures...)
}

func (g GitHub) reconcileConsumer(ctx context.Context, pr Pull) error {
	data, _, err := g.File(ctx, DependencyPath, pr.Head.SHA)
	if missing(err) {
		return g.status(ctx, pr.Head.SHA, "success", "No SDK dependency request")
	}
	if err != nil {
		return err
	}
	d, err := Decode[Dependency](data)
	if err != nil {
		return err
	}
	if err = d.Validate(); err != nil {
		return err
	}
	if d.Channel == "preview" {
		return g.status(ctx, pr.Head.SHA, "pending", "Preview dependency; run sdk-tools sdk promote")
	}
	upstream := GitHub{Runner: g.Runner, Repo: Repository}
	ready, err := upstream.ResolveDependency(ctx, d)
	if err != nil {
		return err
	}
	if ready == nil {
		if err = upstream.RequestRelease(ctx, d.PR); err != nil {
			return err
		}
		return g.status(ctx, pr.Head.SHA, "pending", "Waiting for a verified SDK release")
	}
	tmp, err := os.MkdirTemp("", "sdk-consumer-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if _, err = g.Runner.Run(ctx, "", nil, "gh", "repo", "clone", g.Repo, tmp, "--", "--no-checkout", "--quiet"); err != nil {
		return err
	}
	if _, err = g.Runner.Run(ctx, tmp, nil, "git", "checkout", "--detach", pr.Head.SHA); err != nil {
		return err
	}
	changed, err := upstream.Finalize(ctx, tmp)
	if err != nil {
		return err
	}
	if !changed {
		if err = VerifyLocks(ctx, g.Runner, tmp, d); err != nil {
			return err
		}
		return g.status(ctx, pr.Head.SHA, "success", "Verified SDK release pinned in production locks")
	}
	paths := []string{DependencyPath, filepath.ToSlash(filepath.Join(d.GoDir, "go.mod")), filepath.ToSlash(filepath.Join(d.GoDir, "go.sum"))}
	if d.WebDir != "" {
		paths = append(paths, filepath.ToSlash(filepath.Join(d.WebDir, "package.json")), filepath.ToSlash(filepath.Join(d.WebDir, "pnpm-lock.yaml")))
	}
	files := map[string][]byte{}
	for _, path := range paths {
		contents, err := os.ReadFile(filepath.Join(tmp, path))
		if err != nil {
			return err
		}
		files[path] = contents
	}
	sha, err := g.CommitFiles(ctx, pr.Head.SHA, "chore: pin verified SDK v"+ready.Version, files)
	if err != nil {
		return err
	}
	// A concurrent push makes this non-fast-forward and is retried on the next
	// reconciliation. Never force-update the agent's branch.
	if err = g.API(ctx, "PATCH", "git/refs/heads/"+pr.Head.Ref, map[string]any{"sha": sha, "force": false}, nil); err != nil {
		return err
	}
	return g.status(ctx, sha, "success", "Verified SDK release pinned; consumer CI runs on this commit")
}

func VerifyLocks(ctx context.Context, runner Runner, root string, d Dependency) error {
	if !versionPattern.MatchString(d.Version) || !shaPattern.MatchString(d.SHA) {
		return fmt.Errorf("missing production release identity")
	}
	goDir, err := safeDir(root, d.GoDir)
	if err != nil {
		return err
	}
	out, err := runner.Run(ctx, goDir, nil, "env", "GOWORK=off", "go", "list", "-m", "-json", Repository)
	if err != nil {
		return err
	}
	var module struct {
		Version string
		Replace json.RawMessage
	}
	if err = json.Unmarshal(out, &module); err != nil {
		return err
	}
	if module.Version != "v"+d.Version || len(module.Replace) > 0 {
		return fmt.Errorf("Go production graph does not match the verified release")
	}
	if d.WebDir != "" {
		webDir, err := safeDir(root, d.WebDir)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(webDir, "package.json"))
		if err != nil {
			return err
		}
		var pkg struct {
			Dependencies map[string]string `json:"dependencies"`
		}
		if err = json.Unmarshal(data, &pkg); err != nil {
			return err
		}
		if pkg.Dependencies["@iota-uz/sdk"] != d.Version {
			return fmt.Errorf("npm production version must exactly match Go")
		}
		// Validate that the lockfile agrees without executing consumer scripts.
		if _, err = runner.Run(ctx, webDir, nil, "pnpm", "install", "--ignore-workspace", "--ignore-scripts", "--frozen-lockfile", "--lockfile-only"); err != nil {
			return err
		}
	}
	return nil
}

func ReadyMarker(version string) string { return "sdk-ready:v" + strings.TrimPrefix(version, "v") }
