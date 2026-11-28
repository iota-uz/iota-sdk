package openbao

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
)

type writer struct{ c *Client }
type reader struct{ c *Client }
type metadataReader struct{ c *Client }

type store struct {
	writer
	reader
	metadataReader
}

var (
	_ secrets.Writer         = writer{}
	_ secrets.Reader         = reader{}
	_ secrets.MetadataReader = metadataReader{}
	_ secrets.Store          = store{}
)

// errWriteFenced: a Put that would start after its ref's NotAfter; nothing was sent.
var errWriteFenced = errors.New("the write's deadline has passed")

func (c *Client) kvPath(kind, path string) string {
	segs := strings.Split(path, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return c.mount + "/" + kind + "/" + strings.Join(segs, "/")
}

// Retention of a path's versions: the published version (and the one before it, which Prune leaves to
// readers) must neither be evicted by max_versions nor expire by delete_version_after.
const (
	maxVersions = 100
	// putAttempts bounds the check-and-set retries of a Put racing other writes of the path.
	putAttempts = 8
	// mountCheckEvery is how long a verified mount configuration is trusted before it is read again.
	mountCheckEvery = time.Minute
)

var (
	errRetention = errors.New("the path's version retention is not what the adapter set")
	errMountTTL  = errors.New("the mount expires versions (delete_version_after is not 0)")
	errEvicting  = errors.New("the write would evict a version that is not destroyed yet")
	errRacing    = errors.New("the value is written concurrently")
)

func (w writer) Put(ctx context.Context, ref secrets.Ref, data map[string]string) (secrets.Metadata, error) {
	c := w.c
	if err := c.checkWrite(ref, false); err != nil {
		return secrets.Metadata{}, err
	}
	if data == nil {
		data = map[string]string{}
	}
	if !ref.NotAfter.IsZero() {
		// An admission deadline: nothing is sent after it, and the request gives up at it.
		if !c.now().Before(ref.NotAfter) {
			return secrets.Metadata{}, unavailable("write", errWriteFenced)
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, ref.NotAfter)
		defer cancel()
	}
	if err := c.checkMount(ctx); err != nil {
		return secrets.Metadata{}, err
	}
	for range putAttempts {
		md, err := c.metadata(ctx, ref)
		switch {
		case errors.Is(err, secrets.ErrNotFound):
			// A new path: its retention first, so no version of it is ever written under the defaults.
			if err := c.setRetention(ctx, ref); err != nil {
				return secrets.Metadata{}, err
			}
			md = kvMetadata{MaxVersions: maxVersions, DeleteVersionAfter: "0s"}
		case err != nil:
			return secrets.Metadata{}, err
		}
		if md.MaxVersions != maxVersions || !zeroDuration(md.DeleteVersionAfter) {
			return secrets.Metadata{}, unavailable("write", errRetention)
		}
		// The oldest version not destroyed must survive the write: KV v2 evicts the oldest beyond max_versions.
		oldestLive := 0
		for v, st := range md.Versions {
			if !st.Destroyed && (oldestLive == 0 || v < oldestLive) {
				oldestLive = v
			}
		}
		if oldestLive > 0 && md.CurrentVersion+1-oldestLive+1 > maxVersions {
			return secrets.Metadata{}, unavailable("write", errEvicting)
		}
		resp, err := c.do(ctx, "write", http.MethodPost, c.kvPath("data", ref.Path), nil,
			map[string]any{"data": data, "options": map[string]any{"cas": md.CurrentVersion}})
		if err != nil {
			return secrets.Metadata{}, err
		}
		if resp.status == http.StatusBadRequest && isCASMismatch(resp.body) {
			continue // another write of the path came first: read its metadata again
		}
		if resp.status/100 != 2 {
			return secrets.Metadata{}, classify("write", resp.status)
		}
		var out struct {
			Data struct {
				Version     int       `json:"version"`
				CreatedTime time.Time `json:"created_time"`
			} `json:"data"`
		}
		if json.Unmarshal(resp.body, &out) != nil || out.Data.Version < 1 {
			return secrets.Metadata{}, unavailable("write", errors.New("the response does not decode"))
		}
		// The version this call wrote (KV v2 answers it), never the latest: the caller publishes exactly
		// what it wrote.
		created := md.CreatedTime
		if created.IsZero() {
			created = out.Data.CreatedTime
		}
		return secrets.Metadata{Version: out.Data.Version, CreatedAt: created, UpdatedAt: out.Data.CreatedTime}, nil
	}
	return secrets.Metadata{}, unavailable("write", errRacing)
}

// isCASMismatch reports whether a 400 is KV v2's check-and-set refusal. The body is only matched, never
// quoted (OpenBao's errors name no value either).
func isCASMismatch(body []byte) bool {
	var e struct {
		Errors []string `json:"errors"`
	}
	if json.Unmarshal(body, &e) != nil {
		return false
	}
	for _, m := range e.Errors {
		if strings.Contains(m, "check-and-set") {
			return true
		}
	}
	return false
}

func zeroDuration(s string) bool {
	if s == "" {
		return true
	}
	d, err := time.ParseDuration(s)
	return err == nil && d == 0
}

// setRetention writes a new path's metadata: max_versions = 100 and no expiry.
func (c *Client) setRetention(ctx context.Context, ref secrets.Ref) error {
	resp, err := c.do(ctx, "metadata write", http.MethodPost, c.kvPath("metadata", ref.Path), nil,
		map[string]any{"max_versions": maxVersions, "delete_version_after": "0s"})
	if err != nil {
		return err
	}
	if resp.status/100 != 2 {
		return classify("metadata write", resp.status)
	}
	return nil
}

// checkMount verifies (at most once a minute) that the mount does not expire versions: the smallest
// non-zero delete_version_after of the mount and the path applies, so 0 on the path is not enough.
func (c *Client) checkMount(ctx context.Context) error {
	c.mountMu.Lock()
	defer c.mountMu.Unlock()
	if !c.mountOK.IsZero() && c.now().Sub(c.mountOK) < mountCheckEvery {
		return nil
	}
	resp, err := c.do(ctx, "config", http.MethodGet, c.mount+"/config", nil, nil)
	if err != nil {
		return err
	}
	if resp.status/100 != 2 {
		return classify("config", resp.status)
	}
	var body struct {
		Data struct {
			DeleteVersionAfter string `json:"delete_version_after"`
		} `json:"data"`
	}
	if json.Unmarshal(resp.body, &body) != nil {
		return unavailable("config", errors.New("the response does not decode"))
	}
	if !zeroDuration(body.Data.DeleteVersionAfter) {
		c.mountOK = time.Time{}
		return unavailable("config", errMountTTL)
	}
	c.mountOK = c.now()
	return nil
}

func (w writer) Delete(ctx context.Context, ref secrets.Ref) error {
	c := w.c
	if err := c.checkWrite(ref, false); err != nil {
		return err
	}
	resp, err := c.do(ctx, "delete", http.MethodDelete, c.kvPath("metadata", ref.Path), nil, nil)
	if err != nil {
		return err
	}
	if resp.status/100 == 2 || resp.status == http.StatusNotFound {
		return nil
	}
	return classify("delete", resp.status)
}

func (w writer) Prune(ctx context.Context, ref secrets.Ref, keepFrom int) error {
	c := w.c
	if err := c.checkWrite(ref, false); err != nil {
		return err
	}
	if keepFrom <= 1 {
		return nil
	}
	md, err := c.metadata(ctx, ref)
	if errors.Is(err, secrets.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	var versions []int
	for v, st := range md.Versions {
		if v < keepFrom && !st.Destroyed {
			versions = append(versions, v)
		}
	}
	if len(versions) == 0 {
		return nil
	}
	sort.Ints(versions)
	resp, err := c.do(ctx, "destroy", http.MethodPost, c.kvPath("destroy", ref.Path), nil, map[string]any{"versions": versions})
	if err != nil {
		return err
	}
	if resp.status/100 == 2 || resp.status == http.StatusNotFound {
		return nil
	}
	return classify("destroy", resp.status)
}

func (w writer) List(ctx context.Context, root secrets.Ref) ([]string, error) {
	c := w.c
	if err := c.checkWrite(root, true); err != nil {
		return nil, err
	}
	// KV v2 lists a folder: the root's folder, filtered by what follows its last "/".
	dir, rest := root.Path, ""
	if i := strings.LastIndex(dir, "/"); i < len(dir)-1 {
		dir, rest = dir[:i+1], dir[i+1:]
	}
	var out []string
	if err := c.listDir(ctx, dir, func(key string) bool { return strings.HasPrefix(key, rest) }, &out, 0); err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// maxListDepth bounds the folders a List descends into.
const maxListDepth = 16

func (c *Client) listDir(ctx context.Context, dir string, keep func(string) bool, out *[]string, depth int) error {
	if depth > maxListDepth {
		return unavailable("list", errors.New("the folders nest too deep"))
	}
	resp, err := c.do(ctx, "list", http.MethodGet, c.kvPath("metadata", strings.TrimSuffix(dir, "/"))+"/", url.Values{"list": {"true"}}, nil)
	if err != nil {
		return err
	}
	if resp.status == http.StatusNotFound {
		return nil
	}
	if resp.status/100 != 2 {
		return classify("list", resp.status)
	}
	var body struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	if json.Unmarshal(resp.body, &body) != nil {
		return unavailable("list", errors.New("the response does not decode"))
	}
	for _, k := range body.Data.Keys {
		if keep != nil && !keep(k) {
			continue
		}
		if strings.HasSuffix(k, "/") {
			if err := c.listDir(ctx, dir+k, nil, out, depth+1); err != nil {
				return err
			}
			continue
		}
		*out = append(*out, dir+k)
	}
	return nil
}

func (r reader) Read(ctx context.Context, ref secrets.Ref) (secrets.Value, error) {
	c := r.c
	if err := c.checkRead(ref); err != nil {
		return secrets.Value{}, err
	}
	if ref.Version < 0 {
		return secrets.Value{}, secrets.ErrNotFound
	}
	var q url.Values
	if ref.Version > 0 {
		q = url.Values{"version": {strconv.Itoa(ref.Version)}}
	}
	// One request: the fields and the version they belong to are one KV v2 version.
	resp, err := c.do(ctx, "read", http.MethodGet, c.kvPath("data", ref.Path), q, nil)
	if err != nil {
		return secrets.Value{}, err
	}
	if resp.status/100 != 2 {
		return secrets.Value{}, classify("read", resp.status)
	}
	var body struct {
		Data struct {
			Data     map[string]json.RawMessage `json:"data"`
			Metadata struct {
				Version   int  `json:"version"`
				Destroyed bool `json:"destroyed"`
			} `json:"metadata"`
		} `json:"data"`
	}
	if json.Unmarshal(resp.body, &body) != nil {
		return secrets.Value{}, unavailable("read", errors.New("the response does not decode"))
	}
	md := body.Data.Metadata
	if body.Data.Data == nil || md.Destroyed {
		return secrets.Value{}, secrets.ErrNotFound
	}
	if md.Version < 1 || (ref.Version > 0 && md.Version != ref.Version) {
		return secrets.Value{}, unavailable("read", errors.New("the store answered another version"))
	}
	data := make(map[string]string, len(body.Data.Data))
	for k, raw := range body.Data.Data {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			// A field written outside the port (a number, an object): its JSON, as KV v2 holds it.
			s = string(raw)
		}
		data[k] = s
	}
	return secrets.Value{Data: data, Version: md.Version}, nil
}

func (m metadataReader) Metadata(ctx context.Context, ref secrets.Ref) (secrets.Metadata, error) {
	c := m.c
	if err := c.checkRead(ref); err != nil {
		return secrets.Metadata{}, err
	}
	md, err := c.metadata(ctx, ref)
	if err != nil {
		return secrets.Metadata{}, err
	}
	if md.CurrentVersion < 1 {
		return secrets.Metadata{}, secrets.ErrNotFound
	}
	out := secrets.Metadata{Version: md.CurrentVersion, CreatedAt: md.CreatedTime, UpdatedAt: md.UpdatedTime}
	if v, ok := md.Versions[md.CurrentVersion]; ok && !v.CreatedTime.IsZero() {
		out.UpdatedAt = v.CreatedTime
	}
	return out, nil
}

type versionState struct {
	CreatedTime time.Time `json:"created_time"`
	Destroyed   bool      `json:"destroyed"`
}

type kvMetadata struct {
	CurrentVersion     int
	MaxVersions        int
	DeleteVersionAfter string
	CreatedTime        time.Time
	UpdatedTime        time.Time
	Versions           map[int]versionState
}

func (c *Client) metadata(ctx context.Context, ref secrets.Ref) (kvMetadata, error) {
	resp, err := c.do(ctx, "metadata", http.MethodGet, c.kvPath("metadata", ref.Path), nil, nil)
	if err != nil {
		return kvMetadata{}, err
	}
	if resp.status/100 != 2 {
		return kvMetadata{}, classify("metadata", resp.status)
	}
	var body struct {
		Data struct {
			CurrentVersion     int                     `json:"current_version"`
			MaxVersions        int                     `json:"max_versions"`
			DeleteVersionAfter string                  `json:"delete_version_after"`
			CreatedTime        time.Time               `json:"created_time"`
			UpdatedTime        time.Time               `json:"updated_time"`
			Versions           map[string]versionState `json:"versions"`
		} `json:"data"`
	}
	if json.Unmarshal(resp.body, &body) != nil {
		return kvMetadata{}, unavailable("metadata", errors.New("the response does not decode"))
	}
	out := kvMetadata{
		CurrentVersion: body.Data.CurrentVersion, MaxVersions: body.Data.MaxVersions,
		DeleteVersionAfter: body.Data.DeleteVersionAfter, CreatedTime: body.Data.CreatedTime, UpdatedTime: body.Data.UpdatedTime,
		Versions: make(map[int]versionState, len(body.Data.Versions)),
	}
	for k, v := range body.Data.Versions {
		n, err := strconv.Atoi(k)
		if err != nil {
			return kvMetadata{}, unavailable("metadata", fmt.Errorf("version %q is not a number", k))
		}
		out.Versions[n] = v
	}
	return out, nil
}
