package manage

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// KitScope is the npm scope of kit modules.
const KitScope = "@iota-kit/"

// kitExcluded lists the client-graph resource kinds: pages, navigation,
// cells and shell renderers belong to the browser graph and are not contracts
// another package can import.
var kitExcluded = map[manifest.ResourceKind]bool{
	manifest.ResPage: true, manifest.ResNavigation: true, manifest.ResSlot: true,
	manifest.ResCell: true, manifest.ResShell: true,
}

type kitDoc struct {
	Hash    string                       `json:"hash"`
	Modules map[string]map[string][]byte `json:"modules"`
}

func kitContentHash(k build.Kit) string {
	h := sha256.New()
	names := make([]string, 0, len(k.Modules))
	for n := range k.Modules {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		_, _ = h.Write([]byte(n))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(artifact.TreeHash(k.Modules[n])))
		_, _ = h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// kitSources collects what the kit is generated from: the native descriptors
// plus the pure contracts (and the built contracts.mjs) of every ACTIVE
// package version. Handlers never enter the kit: only manifest resources of
// contract kinds and the contracts graph artifact are used.
func (s *Service) kitSources(ctx context.Context) ([]build.KitSource, error) {
	sources := s.catalog.Natives().Sources()
	active, err := s.catalog.Active(ctx)
	if err != nil {
		return nil, err
	}
	for _, av := range active {
		src := build.KitSource{Namespace: av.Namespace}
		for _, r := range av.Manifest.Resources {
			if kitExcluded[r.Kind] {
				continue
			}
			if r.ContractHash == "" {
				r.ContractHash = manifest.ContractHashOf(r)
			}
			src.Resources = append(src.Resources, r)
		}
		mod, err := s.arts.ReadFile(ctx, av.ArtifactHash, "contracts.mjs")
		if err != nil && serrors.CodeOf(err) != serrors.NotFound {
			return nil, err
		}
		src.ContractsModule = mod
		sources = append(sources, src)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Namespace < sources[j].Namespace })
	return sources, nil
}

// Kit returns the developer kit for the current catalog: native descriptors
// plus the contracts of all active packages, rendered by the kit generator.
// The snapshot is content-hashed and stored in catalog_snapshots. The result
// is cached until the set of sources changes.
func (s *Service) Kit(ctx context.Context) (build.Kit, error) {
	const op = serrors.Op("extruntime.manage.Kit")
	if s.kits == nil {
		return build.Kit{}, serrors.New(serrors.FailedPrecondition, "no kit generator is configured").WithOp(op)
	}
	sources, err := s.kitSources(ctx)
	if err != nil {
		return build.Kit{}, err
	}
	raw, err := json.Marshal(sources)
	if err != nil {
		return build.Kit{}, serrors.Wrap(op, err)
	}
	sum := sha256.Sum256(raw)
	key := hex.EncodeToString(sum[:])

	s.kitMu.Lock()
	defer s.kitMu.Unlock()
	if s.kitKey == key && s.kitCache.Hash != "" {
		return s.kitCache, nil
	}
	kit, err := s.kits.GenerateKit(ctx, sources)
	if err != nil {
		return build.Kit{}, serrors.Wrap(op, err)
	}
	kit.Hash = kitContentHash(kit)
	content, err := json.Marshal(kitDoc{Hash: kit.Hash, Modules: toModules(kit)})
	if err != nil {
		return build.Kit{}, serrors.Wrap(op, err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO extruntime.catalog_snapshots (hash, content) VALUES ($1, $2)
		ON CONFLICT (hash) DO NOTHING`, kit.Hash, content); err != nil {
		return build.Kit{}, serrors.FromDB(op, err)
	}
	s.kitKey, s.kitCache = key, kit
	return kit, nil
}

func toModules(k build.Kit) map[string]map[string][]byte {
	out := make(map[string]map[string][]byte, len(k.Modules))
	for n, t := range k.Modules {
		out[n] = map[string][]byte(t)
	}
	return out
}

// KitSnapshot loads a stored kit snapshot (the contract resolution snapshot a
// version was built against).
func (s *Service) KitSnapshot(ctx context.Context, actor spi.Subject, hash string) (build.Kit, error) {
	if err := require(actor, PermRead); err != nil {
		return build.Kit{}, err
	}
	return s.loadSnapshot(ctx, hash)
}

func (s *Service) loadSnapshot(ctx context.Context, hash string) (build.Kit, error) {
	const op = serrors.Op("extruntime.manage.loadSnapshot")
	var content []byte
	err := s.db.QueryRow(ctx, `SELECT content FROM extruntime.catalog_snapshots WHERE hash = $1`, hash).Scan(&content)
	if errors.Is(err, pgx.ErrNoRows) {
		return build.Kit{}, notFound("kit snapshot " + hash)
	}
	if err != nil {
		return build.Kit{}, serrors.FromDB(op, err)
	}
	var doc kitDoc
	if err := json.Unmarshal(content, &doc); err != nil {
		return build.Kit{}, serrors.Wrap(op, err)
	}
	kit := build.Kit{Hash: doc.Hash, Modules: map[string]build.Tree{}}
	for n, t := range doc.Modules {
		kit.Modules[n] = build.Tree(t)
	}
	return kit, nil
}

// KitModules lists the module names of the current kit and its hash.
func (s *Service) KitModules(ctx context.Context, actor spi.Subject) (hash string, modules []string, err error) {
	if err := require(actor, PermRead); err != nil {
		return "", nil, err
	}
	kit, err := s.Kit(ctx)
	if err != nil {
		return "", nil, err
	}
	for n := range kit.Modules {
		modules = append(modules, n)
	}
	sort.Strings(modules)
	return kit.Hash, modules, nil
}

// KitTarball packs one kit module as an npm-installable tarball (`npm install
// ./iota-kit-<ns>.tgz`). kitHash selects a stored snapshot; empty means the
// current kit. module is "@iota-kit/<namespace>" or just the namespace.
func (s *Service) KitTarball(ctx context.Context, actor spi.Subject, kitHash, module string) (Archive, error) {
	const op = serrors.Op("extruntime.manage.KitTarball")
	if !strings.HasPrefix(module, "@") {
		module = KitScope + module
	}
	e := &auditEntry{Action: ActionKit, Target: module, Detail: map[string]any{"kitHash": kitHash}}
	var out Archive
	err := s.observe(ctx, actor, PermRead, e, func() error {
		var kit build.Kit
		var err error
		if kitHash == "" {
			kit, err = s.Kit(ctx)
		} else {
			kit, err = s.loadSnapshot(ctx, kitHash)
		}
		if err != nil {
			return err
		}
		e.Detail["kitHash"] = kit.Hash
		tree, ok := kit.Modules[module]
		if !ok {
			return notFound("kit module " + module)
		}
		files := make(build.Tree, len(tree)+1)
		for p, c := range tree {
			files["package/"+p] = c
		}
		if _, has := tree["package.json"]; !has {
			pj, _ := json.MarshalIndent(map[string]any{
				"name": module, "version": "0.0.0-kit-" + kit.Hash[:12], "type": "module",
				"main": "index.js", "types": "index.d.ts",
				"exports": map[string]any{".": map[string]string{"types": "./index.d.ts", "default": "./index.js"}},
			}, "", "  ")
			files["package/package.json"] = append(pj, '\n')
		}
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		for _, p := range artifact.Paths(files) {
			if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: p, Mode: 0o644, Size: int64(len(files[p])), Format: tar.FormatPAX}); err != nil {
				return serrors.Wrap(op, err)
			}
			if _, err := tw.Write(files[p]); err != nil {
				return serrors.Wrap(op, err)
			}
		}
		if err := tw.Close(); err != nil {
			return serrors.Wrap(op, err)
		}
		if err := gz.Close(); err != nil {
			return serrors.Wrap(op, err)
		}
		name := strings.NewReplacer("@", "", "/", "-").Replace(module)
		out = Archive{Filename: name + ".tgz", ContentType: "application/gzip", Data: buf.Bytes(), TreeHash: artifact.TreeHash(tree)}
		return nil
	})
	return out, err
}
