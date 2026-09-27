package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DependencyPath = ".sdk/dependency.json"

func ReadDependency(root string) (Dependency, error) {
	data, err := os.ReadFile(filepath.Join(root, DependencyPath))
	if err != nil {
		return Dependency{}, err
	}
	d, err := Decode[Dependency](data)
	if err == nil {
		err = d.Validate()
	}
	return d, err
}

func WriteDependency(root string, d Dependency) error {
	if err := d.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, ".sdk"), 0755); err != nil {
		return err
	}
	if _, err := backupFiles(root, []string{DependencyPath}); err != nil {
		return err
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, DependencyPath), append(data, '\n'), 0644)
}

func safeDir(root, dir string) (string, error) {
	resolved, err := filepath.EvalSymlinks(filepath.Join(root, dir))
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("dependency directory escapes repository")
	}
	return resolved, nil
}

func (g GitHub) Preview(ctx context.Context, root string, d Dependency) (resultErr error) {
	if err := d.Validate(); err != nil {
		return err
	}
	pr, err := g.Pull(ctx, d.PR)
	if err != nil {
		return err
	}
	if pr.Head.Repo.FullName != g.Repo {
		return fmt.Errorf("preview requires a same-repository SDK PR")
	}
	if !shaPattern.MatchString(pr.Head.SHA) || (pr.State == "closed" && !pr.Merged) {
		return fmt.Errorf("SDK PR has no usable preview")
	}
	goDir, err := safeDir(root, d.GoDir)
	if err != nil {
		return err
	}
	workspace := filepath.Join(goDir, "go.work")
	marker := "// Managed by sdk-tools sdk preview\n"
	if data, err := os.ReadFile(workspace); err == nil && !strings.HasPrefix(string(data), marker) {
		return fmt.Errorf("existing go.work is not managed by sdk-tools; refusing to overwrite it")
	}
	cache := filepath.Join(root, ".sdk", "cache", pr.Head.SHA)
	if err := os.MkdirAll(cache, 0755); err != nil {
		return err
	}
	if _, err = safeDir(root, filepath.Join(".sdk", "cache", pr.Head.SHA)); err != nil {
		return err
	}
	// Local exclusions also work for consumers that have not adopted a gitignore.
	for _, pattern := range []string{".sdk/cache/", "go.work", "go.work.sum"} {
		path, err := g.Runner.Run(ctx, root, nil, "git", "rev-parse", "--git-path", "info/exclude")
		if err != nil {
			return err
		}
		exclude := strings.TrimSpace(string(path))
		if !filepath.IsAbs(exclude) {
			exclude = filepath.Join(root, exclude)
		}
		if err = os.MkdirAll(filepath.Dir(exclude), 0755); err != nil {
			return err
		}
		data, _ := os.ReadFile(exclude)
		if !strings.Contains("\n"+string(data), "\n"+pattern+"\n") {
			if err = os.WriteFile(exclude, append(data, []byte("\n"+pattern+"\n")...), 0644); err != nil {
				return err
			}
		}
	}
	if _, err = g.Runner.Run(ctx, cache, nil, "git", "init", "-q"); err != nil {
		return err
	}
	if _, err = g.Runner.Run(ctx, cache, nil, "git", "fetch", "--depth=1", "https://github.com/"+Repository+".git", pr.Head.SHA); err != nil {
		return err
	}
	if _, err = g.Runner.Run(ctx, cache, nil, "git", "checkout", "--detach", pr.Head.SHA); err != nil {
		return err
	}
	if err = os.Remove(workspace); err != nil && !os.IsNotExist(err) {
		return err
	}
	if _, err = g.Runner.Run(ctx, goDir, nil, "go", "work", "init", goDir, cache); err != nil {
		return err
	}
	data, err := os.ReadFile(workspace)
	if err != nil {
		return err
	}
	if err = os.WriteFile(workspace, append([]byte(marker), data...), 0644); err != nil {
		return err
	}
	if d.WebDir == "" {
		return nil
	}
	webDir, err := safeDir(root, d.WebDir)
	if err != nil {
		return err
	}
	// The consumer's production lockfiles remain untouched. Only node_modules
	// receives the local preview; CI repeats this from the committed declaration.
	artifactDir := filepath.Join(cache, "artifacts", "frontend")
	if err = os.RemoveAll(artifactDir); err != nil {
		return err
	}
	if err = os.MkdirAll(artifactDir, 0755); err != nil {
		return err
	}
	var runs []struct {
		DatabaseID int    `json:"databaseId"`
		Conclusion string `json:"conclusion"`
		HeadSHA    string `json:"headSha"`
	}
	out, err := g.runner().Run(ctx, "", nil, "gh", "run", "list", "--repo", Repository, "--workflow", "frontend-packages.yml", "--commit", pr.Head.SHA, "--limit", "20", "--json", "databaseId,conclusion,headSha")
	if err != nil {
		return err
	}
	if err = json.Unmarshal(out, &runs); err != nil {
		return err
	}
	runID := 0
	for _, run := range runs {
		if run.Conclusion == "success" && run.HeadSHA == pr.Head.SHA {
			runID = run.DatabaseID
			break
		}
	}
	if runID == 0 {
		return fmt.Errorf("no successful frontend preview artifact exists for SDK PR head %s", pr.Head.SHA)
	}
	if _, err = g.runner().Run(ctx, "", nil, "gh", "run", "download", fmt.Sprint(runID), "--repo", Repository, "--name", "frontend-"+pr.Head.SHA, "--dir", artifactDir); err != nil {
		return err
	}
	manifestData, err := os.ReadFile(filepath.Join(artifactDir, "frontend-artifacts.json"))
	if err != nil {
		return err
	}
	var manifest struct {
		File string `json:"file"`
		SHA  string `json:"sdkCommit"`
	}
	if err = json.Unmarshal(manifestData, &manifest); err != nil {
		return err
	}
	if manifest.SHA != pr.Head.SHA || filepath.Base(manifest.File) != manifest.File {
		return fmt.Errorf("preview artifact identity mismatch")
	}
	backups, err := backupFiles(root, []string{filepath.Join(d.WebDir, "package.json"), filepath.Join(d.WebDir, "pnpm-lock.yaml")})
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, restoreFiles(backups))
	}()
	_, err = g.Runner.Run(ctx, webDir, nil, "pnpm", "add", "--ignore-workspace", "--ignore-scripts", "--ignore-pnpmfile", "--save-exact", filepath.Join(artifactDir, manifest.File))
	return err
}

// Promote resumes remote release work and only modifies the local consumer.
func (g GitHub) Promote(ctx context.Context, root string, retry bool, out io.Writer) error {
	d, err := ReadDependency(root)
	if err != nil {
		return err
	}
	pr, err := g.Pull(ctx, d.PR)
	if err != nil {
		return err
	}
	if !pr.Merged {
		return fmt.Errorf("merge SDK PR #%d before promoting", d.PR)
	}
	dispatched := false
	for {
		ready, err := g.ResolveDependency(ctx, d)
		if err != nil {
			return err
		}
		if ready != nil {
			break
		}
		var runs struct {
			Runs []struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
				URL    string `json:"html_url"`
			} `json:"workflow_runs"`
		}
		if err = g.API(ctx, "GET", "actions/workflows/release.yml/runs?branch=main&event=workflow_dispatch&per_page=100", nil, &runs); err != nil {
			return err
		}
		active := false
		for _, run := range runs.Runs {
			if run.Status != "completed" {
				active = true
				fmt.Fprintf(out, "Waiting for SDK release: %s\n", run.URL)
				if _, err = g.runner().Run(ctx, "", nil, "gh", "run", "watch", fmt.Sprint(run.ID), "--repo", g.Repo, "--exit-status"); err != nil {
					return fmt.Errorf("SDK release failed or waiting was interrupted; rerun promote to resume (use --retry after fixing infrastructure): %w", err)
				}
				break
			}
		}
		if active {
			continue
		}
		if dispatched {
			return fmt.Errorf("release finished without a ready version containing SDK PR #%d; inspect release.yml and rerun promote", d.PR)
		}
		state, _, err := g.ReadState(ctx)
		if err != nil {
			return err
		}
		if state.Candidate != nil && state.Candidate.Phase == "failed" && !retry {
			main, err := g.Ref(ctx, "main")
			if err != nil {
				return err
			}
			if main == state.Candidate.Source {
				return fmt.Errorf("SDK verification failed; fix SDK or use promote --retry after an infrastructure fix")
			}
		}
		if err = g.API(ctx, "POST", "actions/workflows/release.yml/dispatches", map[string]any{"ref": "main", "inputs": map[string]string{"sdk_pr": fmt.Sprint(d.PR), "retry": fmt.Sprint(retry)}}, nil); err != nil {
			return err
		}
		fmt.Fprintln(out, "SDK release requested. Rerun promote if this session is interrupted.")
		dispatched = true
		// Dispatch becomes visible asynchronously; wait for a run or ready state.
		for attempt := 0; attempt < 12; attempt++ {
			if err = g.API(ctx, "GET", "actions/workflows/release.yml/runs?branch=main&event=workflow_dispatch&per_page=100", nil, &runs); err != nil {
				return err
			}
			found := false
			for _, run := range runs.Runs {
				if run.Status != "completed" {
					found = true
					break
				}
			}
			if found {
				break
			}
			timer := time.NewTimer(5 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	_, err = g.Finalize(ctx, root)
	return err
}

func (g GitHub) Resolve(ctx context.Context, number int) (*Candidate, error) {
	pr, err := g.Pull(ctx, number)
	if err != nil {
		return nil, err
	}
	if !pr.Merged {
		if pr.State == "closed" {
			return nil, fmt.Errorf("SDK PR was closed without merging")
		}
		return nil, nil
	}
	state, _, err := g.ReadState(ctx)
	if err != nil || state.Ready == nil {
		return nil, err
	}
	ok, err := g.Contains(ctx, state.Ready.SHA, pr.Merge)
	if !ok || err != nil {
		return nil, err
	}
	actual, err := g.Ref(ctx, "v"+state.Ready.Version)
	if err != nil {
		return nil, err
	}
	if actual != state.Ready.SHA {
		return nil, fmt.Errorf("ready release tag was changed")
	}
	return state.Ready, nil
}

func (g GitHub) ResolveDependency(ctx context.Context, d Dependency) (*Candidate, error) {
	if d.Version == "" && d.SHA == "" {
		return g.Resolve(ctx, d.PR)
	}
	if !versionPattern.MatchString(d.Version) || !shaPattern.MatchString(d.SHA) {
		return nil, fmt.Errorf("invalid pinned release identity")
	}
	state, _, err := g.ReadState(ctx)
	if err != nil {
		return nil, err
	}
	ready := state.Releases[d.Version]
	if ready == nil && state.Ready != nil && state.Ready.Version == d.Version {
		ready = state.Ready
	}
	if ready == nil || ready.SHA != d.SHA || ready.Phase != "ready" {
		return nil, fmt.Errorf("release has no matching completion record")
	}
	sha, err := g.Ref(ctx, "v"+d.Version)
	if err != nil {
		return nil, err
	}
	if sha != d.SHA {
		return nil, fmt.Errorf("release tag differs from the pinned SHA")
	}
	pr, err := g.Pull(ctx, d.PR)
	if err != nil {
		return nil, err
	}
	if !pr.Merged {
		return nil, fmt.Errorf("pinned release requires a merged SDK PR")
	}
	contains, err := g.Contains(ctx, sha, pr.Merge)
	if err != nil {
		return nil, err
	}
	if !contains {
		return nil, fmt.Errorf("pinned release does not include the required SDK PR")
	}
	return &Candidate{Version: d.Version, SHA: sha, Phase: "ready"}, nil
}

func (g GitHub) Finalize(ctx context.Context, root string) (changed bool, resultErr error) {
	d, err := ReadDependency(root)
	if err != nil {
		return false, err
	}
	ready, err := g.ResolveDependency(ctx, d)
	if err != nil {
		return false, err
	}
	if ready == nil {
		return false, fmt.Errorf("no verified release is ready; run sdk-tools sdk promote")
	}
	if d.Version == ready.Version && d.SHA == ready.SHA {
		return false, VerifyLocks(ctx, g.Runner, root, d)
	}
	paths := dependencyFiles(d)
	status, err := g.Runner.Run(ctx, root, nil, "git", append([]string{"status", "--porcelain", "--"}, paths...)...)
	if err != nil {
		return false, err
	}
	if len(status) != 0 {
		return false, fmt.Errorf("commit dependency files before finalizing; refusing to overwrite local work")
	}
	backups, err := backupFiles(root, append(paths, filepath.Join(d.GoDir, "go.work"), filepath.Join(d.GoDir, "go.work.sum")))
	if err != nil {
		return false, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, restoreFiles(backups))
		}
	}()
	goDir, err := safeDir(root, d.GoDir)
	if err != nil {
		return false, err
	}
	if _, err = g.Runner.Run(ctx, goDir, nil, "env", "GOWORK=off", "go", "get", Repository+"@v"+ready.Version); err != nil {
		return false, err
	}
	if d.WebDir != "" {
		webDir, err := safeDir(root, d.WebDir)
		if err != nil {
			return false, err
		}
		if _, err = g.Runner.Run(ctx, webDir, nil, "pnpm", "add", "--ignore-workspace", "--ignore-scripts", "--ignore-pnpmfile", "--save-exact", "@iota-uz/sdk@"+ready.Version); err != nil {
			return false, err
		}
	}
	d.Channel, d.Version, d.SHA = "release", ready.Version, ready.SHA
	if err = VerifyLocks(ctx, g.Runner, root, d); err != nil {
		return false, err
	}
	if _, err = g.Runner.Run(ctx, goDir, nil, "env", "GOWORK=off", "go", "vet", "./..."); err != nil {
		return false, err
	}
	if err = WriteDependency(root, d); err != nil {
		return false, err
	}
	workspace := filepath.Join(goDir, "go.work")
	if data, readErr := os.ReadFile(workspace); readErr == nil && strings.HasPrefix(string(data), "// Managed by sdk-tools sdk preview\n") {
		if err = os.Remove(workspace); err != nil {
			return false, err
		}
		if err = os.Remove(workspace + ".sum"); err != nil && !os.IsNotExist(err) {
			return false, err
		}
	}
	return true, nil

}
