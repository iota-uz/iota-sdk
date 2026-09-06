package sdk

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(ctx context.Context, dir string, input []byte, name string, args ...string) ([]byte, error)
}

type ExecRunner struct{ Token string }

func (r ExecRunner) Run(ctx context.Context, dir string, input []byte, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if name != "gh" || r.Token != "" {
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "GH_TOKEN=") && !strings.HasPrefix(entry, "GITHUB_TOKEN=") && !strings.HasPrefix(entry, "SDK_GH_TOKEN=") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		if name == "gh" {
			cmd.Env = append(cmd.Env, "GH_TOKEN="+r.Token)
		}
	}
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

type GitHub struct {
	Runner Runner
	Repo   string
}

func (g GitHub) API(ctx context.Context, method, endpoint string, body any, result any) error {
	var input []byte
	args := []string{"api", "--method", method, strings.TrimSuffix("repos/"+g.Repo+"/"+endpoint, "/")}
	if body != nil {
		var err error
		input, err = json.Marshal(body)
		if err != nil {
			return err
		}
		args = append(args, "--input", "-")
	}
	runner := g.Runner
	if _, ok := runner.(ExecRunner); ok && g.Repo == Repository && os.Getenv("SDK_GH_TOKEN") != "" {
		runner = ExecRunner{Token: os.Getenv("SDK_GH_TOKEN")}
	}
	out, err := runner.Run(ctx, "", input, "gh", args...)
	if err != nil {
		return err
	}
	if result != nil {
		return json.Unmarshal(out, result)
	}
	return nil
}

func missing(err error) bool {
	return err != nil && strings.Contains(err.Error(), "HTTP 404")
}

type content struct {
	SHA     string `json:"sha"`
	Content string `json:"content"`
}

func (g GitHub) File(ctx context.Context, path, ref string) ([]byte, string, error) {
	var file content
	if err := g.API(ctx, "GET", "contents/"+path+"?ref="+url.QueryEscape(ref), nil, &file); err != nil {
		return nil, "", err
	}
	data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
	return data, file.SHA, err
}

type Pull struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	Merged bool   `json:"merged"`
	Merge  string `json:"merge_commit_sha"`
	Head   struct {
		SHA  string `json:"sha"`
		Ref  string `json:"ref"`
		Repo struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"head"`
}

func (g GitHub) Pull(ctx context.Context, number int) (Pull, error) {
	var pr Pull
	err := g.API(ctx, "GET", fmt.Sprintf("pulls/%d", number), nil, &pr)
	return pr, err
}

func (g GitHub) Ref(ctx context.Context, ref string) (string, error) {
	var value struct {
		SHA string `json:"sha"`
	}
	err := g.API(ctx, "GET", "commits/"+url.PathEscape(ref), nil, &value)
	return value.SHA, err
}

func (g GitHub) Contains(ctx context.Context, descendant, ancestor string) (bool, error) {
	if !shaPattern.MatchString(descendant) || !shaPattern.MatchString(ancestor) {
		return false, fmt.Errorf("comparison requires immutable commit SHAs")
	}
	var comparison struct {
		Status string `json:"status"`
	}
	err := g.API(ctx, "GET", "compare/"+ancestor+"..."+descendant, nil, &comparison)
	return comparison.Status == "ahead" || comparison.Status == "identical", err
}

func (g GitHub) ReadState(ctx context.Context) (State, string, error) {
	data, sha, err := g.File(ctx, "state.json", StateBranch)
	if missing(err) {
		return State{}, "", nil
	}
	if err != nil {
		return State{}, "", err
	}
	state, err := Decode[State](data)
	return state, sha, err
}

func (g GitHub) UpdateState(ctx context.Context, update func(*State) error) error {
	for attempt := 0; attempt < 6; attempt++ {
		state, sha, err := g.ReadState(ctx)
		if err != nil {
			return err
		}
		if err = update(&state); err != nil {
			return err
		}
		if sha == "" {
			main, err := g.Ref(ctx, "main")
			if err != nil {
				return err
			}
			err = g.API(ctx, "POST", "git/refs", map[string]string{"ref": "refs/heads/" + StateBranch, "sha": main}, nil)
			if err != nil && !strings.Contains(err.Error(), "HTTP 422") {
				return err
			}
		}
		data, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return err
		}
		body := map[string]string{"branch": StateBranch, "message": "chore: reconcile SDK release state", "content": base64.StdEncoding.EncodeToString(append(data, '\n'))}
		if sha != "" {
			body["sha"] = sha
		}
		err = g.API(ctx, "PUT", "contents/state.json", body, nil)
		if err == nil {
			return nil
		}
		if !strings.Contains(err.Error(), "HTTP 409") && !strings.Contains(err.Error(), "HTTP 422") {
			return err
		}
	}
	return fmt.Errorf("release state changed repeatedly; retry reconciliation")
}

func (g GitHub) CommitFiles(ctx context.Context, parent, message string, files map[string][]byte) (string, error) {
	var commit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if err := g.API(ctx, "GET", "git/commits/"+parent, nil, &commit); err != nil {
		return "", err
	}
	entries := make([]map[string]string, 0, len(files))
	for path, data := range files {
		entries = append(entries, map[string]string{"path": path, "mode": "100644", "type": "blob", "content": string(data)})
	}
	var tree struct {
		SHA string `json:"sha"`
	}
	if err := g.API(ctx, "POST", "git/trees", map[string]any{"base_tree": commit.Tree.SHA, "tree": entries}, &tree); err != nil {
		return "", err
	}
	var created struct {
		SHA string `json:"sha"`
	}
	err := g.API(ctx, "POST", "git/commits", map[string]any{"message": message, "tree": tree.SHA, "parents": []string{parent}}, &created)
	return created.SHA, err
}
