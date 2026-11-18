package mcptools

import (
	"context"
	"encoding/base64"
	"time"
	"unicode/utf8"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
)

type namespaceIn struct {
	Namespace string `json:"namespace" jsonschema:"package namespace, for example acme.insurance"`
}

type packageList struct {
	Packages []manage.Package `json:"packages"`
}

type createPackageIn struct {
	Namespace string `json:"namespace" jsonschema:"stable dotted owner prefix of every resource ID, for example acme.insurance"`
	NPMName   string `json:"npm_name,omitempty" jsonschema:"npm package name; defaults to a name derived from the namespace"`
}

type fileList struct {
	Files []manage.File `json:"files"`
}

type fileReadIn struct {
	Namespace string `json:"namespace"`
	Path      string `json:"path" jsonschema:"draft file path, for example src/contracts.ts"`
}

type fileContent struct {
	Path      string    `json:"path"`
	Hash      string    `json:"hash" jsonschema:"content hash; pass it as expected_hash when writing the file back"`
	Size      int       `json:"size"`
	UpdatedBy int64     `json:"updatedBy"`
	UpdatedAt time.Time `json:"updatedAt"`
	// Encoding is "utf-8" for text and "base64" for binary content.
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

func contentOf(f manage.File, encoding, content string) fileContent {
	return fileContent{Path: f.Path, Hash: f.Hash, Size: f.Size, UpdatedBy: f.UpdatedBy, UpdatedAt: f.UpdatedAt, Encoding: encoding, Content: content}
}

type fileWriteIn struct {
	Namespace    string `json:"namespace"`
	Path         string `json:"path"`
	Content      string `json:"content"`
	Encoding     string `json:"encoding,omitempty" jsonschema:"utf-8 (default) or base64"`
	ExpectedHash string `json:"expected_hash,omitempty" jsonschema:"content hash the file must still have; the save is refused with a conflict when someone else changed it"`
	CreateOnly   bool   `json:"create_only,omitempty" jsonschema:"require that the file does not exist yet"`
}

type fileDeleteIn struct {
	Namespace string `json:"namespace"`
	Path      string `json:"path"`
}

type fileMoveIn struct {
	Namespace string `json:"namespace"`
	From      string `json:"from"`
	To        string `json:"to"`
}

type importGitIn struct {
	RepoURL string `json:"repo_url" jsonschema:"repository URL allowed by the host's git configuration"`
	Commit  string `json:"commit" jsonschema:"full 40-character commit SHA; branches and tags are not accepted"`
	Subdir  string `json:"subdir,omitempty" jsonschema:"package directory inside the repository"`
}

type exportIn struct {
	Namespace string `json:"namespace"`
	Version   string `json:"version,omitempty" jsonschema:"published version to export; the draft when omitted"`
	Format    string `json:"format,omitempty" jsonschema:"tar (default) or zip"`
}

type exportOut struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	TreeHash    string `json:"tree_hash"`
	Size        int    `json:"size"`
	DataBase64  string `json:"data_base64"`
}

func packageTools(r *registrar, d Deps) {
	svc := d.Manage

	add(r, mcp.Spec{
		Name:        "ext_package_list",
		Description: describe("List extension packages (drafts) with their active version.", manage.PermRead),
		Annotations: mcp.Annotations{Title: "List extension packages", ReadOnly: true, Idempotent: true},
		Policy:      readPolicy(manage.PermRead), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, _ struct{}) (packageList, error) {
		out, err := svc.ListPackages(ctx, actor)
		return packageList{Packages: nonNilSlice(out)}, err
	})

	add(r, mcp.Spec{
		Name:        "ext_package_get",
		Description: describe("Read one package draft head: provenance, draft revision, active version.", manage.PermRead),
		Annotations: mcp.Annotations{Title: "Get extension package", ReadOnly: true, Idempotent: true},
		Policy:      readPolicy(manage.PermRead), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, in namespaceIn) (manage.Package, error) {
		return svc.GetPackage(ctx, actor, in.Namespace)
	})

	add(r, mcp.Spec{
		Name: "ext_package_create",
		Description: describe("Create an empty runtime-issued package draft. Add its files with ext_file_write, "+
			"or import a Git commit with ext_import_git.", manage.PermAuthor),
		Annotations: mcp.Annotations{Title: "Create extension package"},
		Policy:      managePolicy(), Meta: meta(manage.PermAuthor),
	}, func(ctx context.Context, actor spi.Subject, in createPackageIn) (manage.Package, error) {
		return svc.CreatePackage(ctx, actor, in.Namespace, in.NPMName)
	})

	add(r, mcp.Spec{
		Name:        "ext_file_list",
		Description: describe("List the files of a package draft with content hashes.", manage.PermRead),
		Annotations: mcp.Annotations{Title: "List draft files", ReadOnly: true, Idempotent: true},
		Policy:      readPolicy(manage.PermRead), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, in namespaceIn) (fileList, error) {
		out, err := svc.ListFiles(ctx, actor, in.Namespace)
		return fileList{Files: nonNilSlice(out)}, err
	})

	add(r, mcp.Spec{
		Name:        "ext_file_read",
		Description: describe("Read one draft file. Pass the returned hash as expected_hash when writing it back.", manage.PermRead),
		Annotations: mcp.Annotations{Title: "Read draft file", ReadOnly: true, Idempotent: true},
		Policy:      readPolicy(manage.PermRead), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, in fileReadIn) (fileContent, error) {
		content, f, err := svc.GetFile(ctx, actor, in.Namespace, in.Path)
		if err != nil {
			return fileContent{}, err
		}
		if utf8.Valid(content) {
			return contentOf(f, "utf-8", string(content)), nil
		}
		return contentOf(f, "base64", base64.StdEncoding.EncodeToString(content)), nil
	})

	add(r, mcp.Spec{
		Name: "ext_file_write",
		Description: describe("Create or replace one draft file. Concurrent edits are detected: pass expected_hash "+
			"(from ext_file_read or ext_file_list) or create_only.", manage.PermAuthor),
		Annotations: mcp.Annotations{Title: "Write draft file", Idempotent: true},
		Policy:      managePolicy(), Meta: meta(manage.PermAuthor),
	}, func(ctx context.Context, actor spi.Subject, in fileWriteIn) (manage.File, error) {
		content := []byte(in.Content)
		switch in.Encoding {
		case "", "utf-8":
		case "base64":
			raw, err := base64.StdEncoding.DecodeString(in.Content)
			if err != nil {
				return manage.File{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "invalid input",
					mcp.Issue{Path: "content", Code: "base64", Message: "content is not valid base64"})
			}
			content = raw
		default:
			return manage.File{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "invalid input",
				mcp.Issue{Path: "encoding", Code: "enum", Message: "encoding must be utf-8 or base64"})
		}
		var expect *string
		switch {
		case in.ExpectedHash != "" && in.CreateOnly:
			return manage.File{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "expected_hash and create_only exclude each other")
		case in.CreateOnly:
			empty := ""
			expect = &empty
		case in.ExpectedHash != "":
			expect = &in.ExpectedHash
		}
		return svc.PutFile(ctx, actor, in.Namespace, in.Path, content, expect)
	})

	add(r, mcp.Spec{
		Name:        "ext_file_delete",
		Description: describe("Delete one draft file.", manage.PermAuthor),
		Annotations: mcp.Annotations{Title: "Delete draft file", Destructive: true, Idempotent: true},
		Policy:      managePolicy(), Meta: meta(manage.PermAuthor),
	}, func(ctx context.Context, actor spi.Subject, in fileDeleteIn) (ack, error) {
		return done, svc.DeleteFile(ctx, actor, in.Namespace, in.Path)
	})

	add(r, mcp.Spec{
		Name:        "ext_file_move",
		Description: describe("Rename or move one draft file.", manage.PermAuthor),
		Annotations: mcp.Annotations{Title: "Move draft file"},
		Policy:      managePolicy(), Meta: meta(manage.PermAuthor),
	}, func(ctx context.Context, actor spi.Subject, in fileMoveIn) (ack, error) {
		return done, svc.MoveFile(ctx, actor, in.Namespace, in.From, in.To)
	})

	add(r, mcp.Spec{
		Name: "ext_import_git",
		Description: describe("Replace (or create) a package draft with the tree of one Git commit. The draft keeps its "+
			"provenance (repository URL and commit).", manage.PermAuthor),
		Annotations: mcp.Annotations{Title: "Import Git commit", Destructive: true, OpenWorld: true},
		Policy:      managePolicy(), Meta: meta(manage.PermAuthor),
	}, func(ctx context.Context, actor spi.Subject, in importGitIn) (manage.Package, error) {
		return svc.ImportGit(ctx, actor, in.RepoURL, in.Commit, in.Subdir)
	})

	add(r, mcp.Spec{
		Name: "ext_export",
		Description: describe("Export the draft, or the exact source of a published version, as a tar or zip archive "+
			"(base64). Importing it again yields the same tree hash.", manage.PermRead),
		Annotations: mcp.Annotations{Title: "Export package", ReadOnly: true, Idempotent: true},
		Policy:      managePolicy(), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, in exportIn) (exportOut, error) {
		format := manage.ExportFormat(in.Format)
		if format == "" {
			format = manage.FormatTar
		}
		var arc manage.Archive
		var err error
		if in.Version != "" {
			arc, err = svc.ExportVersion(ctx, actor, in.Namespace, in.Version, format)
		} else {
			arc, err = svc.ExportDraft(ctx, actor, in.Namespace, format)
		}
		if err != nil {
			return exportOut{}, err
		}
		return exportOut{
			Filename: arc.Filename, ContentType: arc.ContentType, TreeHash: arc.TreeHash,
			Size: len(arc.Data), DataBase64: base64.StdEncoding.EncodeToString(arc.Data),
		}, nil
	})

	add(r, mcp.Spec{
		Name: "ext_check",
		Description: describe("Typecheck the saved draft and validate its graphs against the current developer kit. "+
			"Diagnostics carry file, line and column; an unsaved buffer is never checked.", manage.PermAuthor),
		Annotations: mcp.Annotations{Title: "Check draft", Idempotent: true},
		Policy:      managePolicy(), Meta: meta(manage.PermAuthor),
	}, func(ctx context.Context, actor spi.Subject, in namespaceIn) (manage.CheckReport, error) {
		out, err := svc.Check(ctx, actor, in.Namespace)
		if out.Diagnostics == nil {
			out.Diagnostics = []build.Diagnostic{}
		}
		return out, err
	})

	add(r, mcp.Spec{
		Name: "ext_build",
		Description: describe("Build the saved draft. Returns diagnostics, the source and artifact hashes and the manifest; "+
			"nothing is published.", manage.PermAuthor),
		Annotations: mcp.Annotations{Title: "Build draft", Idempotent: true},
		Policy:      managePolicy(), Meta: meta(manage.PermAuthor),
	}, func(ctx context.Context, actor spi.Subject, in namespaceIn) (manage.BuildReport, error) {
		out, err := svc.Build(ctx, actor, in.Namespace)
		if out.Diagnostics == nil {
			out.Diagnostics = []build.Diagnostic{}
		}
		return out, err
	})

	add(r, mcp.Spec{
		Name: "ext_test",
		Description: describe("Run the draft's synthetic tests (*.test.ts) against fake effects. An effect without a fake "+
			"fails the test; it never reaches a real external system.", manage.PermAuthor),
		Annotations: mcp.Annotations{Title: "Test draft", Idempotent: true},
		Policy:      managePolicy(), Meta: meta(manage.PermAuthor),
	}, func(ctx context.Context, actor spi.Subject, in namespaceIn) (build.TestReport, error) {
		out, err := svc.Test(ctx, actor, in.Namespace)
		if out.Cases == nil {
			out.Cases = []build.TestCase{}
		}
		return out, err
	})

	add(r, mcp.Spec{
		Name:        "ext_kit_info",
		Description: describe("Describe the developer kit: its snapshot hash and the @iota-kit/* modules (typings and pure refs of core and other packages). Download a module with ext_kit_module, or GET /_ext/manage/kit/modules/<module>.tgz.", manage.PermRead),
		Annotations: mcp.Annotations{Title: "Developer kit", ReadOnly: true, Idempotent: true},
		Policy:      readPolicy(manage.PermRead), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, _ struct{}) (kitInfo, error) {
		hash, modules, err := svc.KitModules(ctx, actor)
		if err != nil {
			return kitInfo{}, err
		}
		out := kitInfo{Hash: hash, Modules: make([]kitModule, 0, len(modules))}
		for _, m := range modules {
			out.Modules = append(out.Modules, kitModule{Name: m, DownloadPath: kitDownloadPath(m, hash)})
		}
		return out, nil
	})

	add(r, mcp.Spec{
		Name:        "ext_kit_module",
		Description: describe("Download one developer kit module as an npm-installable tarball (base64).", manage.PermRead),
		Annotations: mcp.Annotations{Title: "Download kit module", ReadOnly: true, Idempotent: true},
		Policy:      managePolicy(), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, in kitModuleIn) (exportOut, error) {
		arc, err := svc.KitTarball(ctx, actor, in.KitHash, in.Module)
		if err != nil {
			return exportOut{}, err
		}
		return exportOut{
			Filename: arc.Filename, ContentType: arc.ContentType, TreeHash: arc.TreeHash,
			Size: len(arc.Data), DataBase64: base64.StdEncoding.EncodeToString(arc.Data),
		}, nil
	})
}

type kitModuleIn struct {
	Module  string `json:"module" jsonschema:"@iota-kit/<namespace> or just the namespace"`
	KitHash string `json:"kit_hash,omitempty" jsonschema:"stored snapshot to download; the current kit when omitted"`
}

type kitModule struct {
	Name         string `json:"name"`
	DownloadPath string `json:"download_path"`
}

type kitInfo struct {
	Hash    string      `json:"hash"`
	Modules []kitModule `json:"modules"`
}

func kitDownloadPath(module, hash string) string {
	return "/_ext/manage/kit/modules/" + module + ".tgz?hash=" + hash
}

func nonNilSlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
