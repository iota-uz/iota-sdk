package core_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const testBucket = "uploads"

var testTenantID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// memoryS3 is an in-memory S3 API double; reads of unknown keys return NoSuchKey.
type memoryS3 struct {
	mu       sync.Mutex
	objects  map[string][]byte
	types    map[string]string
	requests int
}

func newMemoryS3() *memoryS3 {
	return &memoryS3{objects: map[string][]byte{}, types: map[string]string{}}
}

func (m *memoryS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	data, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests++
	id := aws.ToString(in.Bucket) + "/" + aws.ToString(in.Key)
	m.objects[id] = data
	m.types[id] = aws.ToString(in.ContentType)
	return &s3.PutObjectOutput{}, nil
}

func (m *memoryS3) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests++
	id := aws.ToString(in.Bucket) + "/" + aws.ToString(in.Key)
	data, ok := m.objects[id]
	if !ok {
		return nil, &s3types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   aws.String(m.types[id]),
	}, nil
}

func (m *memoryS3) CopyObject(_ context.Context, in *s3.CopyObjectInput, _ ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	source, err := url.PathUnescape(aws.ToString(in.CopySource))
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests++
	data, ok := m.objects[source]
	if !ok {
		return nil, &s3types.NoSuchKey{}
	}
	id := aws.ToString(in.Bucket) + "/" + aws.ToString(in.Key)
	m.objects[id] = data
	m.types[id] = m.types[source]
	return &s3.CopyObjectOutput{}, nil
}

func (m *memoryS3) DeleteObject(_ context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests++
	delete(m.objects, aws.ToString(in.Bucket)+"/"+aws.ToString(in.Key))
	return &s3.DeleteObjectOutput{}, nil
}

func (m *memoryS3) PresignGetObject(_ context.Context, in *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	options := s3.PresignOptions{}
	for _, fn := range optFns {
		fn(&options)
	}
	return &v4.PresignedHTTPRequest{
		Method: http.MethodGet,
		URL:    fmt.Sprintf("https://minio.test/%s/%s?X-Amz-Expires=%d", aws.ToString(in.Bucket), aws.ToString(in.Key), int(options.Expires.Seconds())),
	}, nil
}

func (m *memoryS3) object(key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[testBucket+"/"+key]
	return data, ok
}

func (m *memoryS3) put(key string, data []byte, contentType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[testBucket+"/"+key] = data
	m.types[testBucket+"/"+key] = contentType
}

func (m *memoryS3) requestCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.requests
}

type dualStorageFixture struct {
	root    string
	cfg     *uploadsconfig.Config
	s3      *memoryS3
	storage *persistence.RoutedStorage
}

func newDualStorageFixture(t *testing.T, s3Primary bool) *dualStorageFixture {
	t.Helper()
	root, err := os.MkdirTemp(".", "upload-dual-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(root)) })
	cfg := &uploadsconfig.Config{Path: filepath.Clean(root)}
	fsStorage, err := persistence.NewFSStorage(cfg)
	require.NoError(t, err)
	memory := newMemoryS3()
	routed, err := persistence.NewRoutedStorage(fsStorage, persistence.NewS3StorageWithClient(testBucket, memory, memory), s3Primary)
	require.NoError(t, err)
	return &dualStorageFixture{root: cfg.Path, cfg: cfg, s3: memory, storage: routed}
}

func readAllAndClose(t *testing.T, body io.ReadCloser) []byte {
	t.Helper()
	defer func() { require.NoError(t, body.Close()) }()
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	return data
}

func TestRoutedStorage_RoutesReadsByStoredPath(t *testing.T) {
	// Falsely green if both backends answered every path: the S3 double must
	// stay untouched for filesystem paths and the disk must lack the S3 object.
	f := newDualStorageFixture(t, false)
	ctx := context.Background()
	fsContent := []byte("legacy filesystem upload")
	fsPath := filepath.Join(f.root, "legacy.txt")
	require.NoError(t, os.WriteFile(fsPath, fsContent, 0o600))
	s3Content := []byte("%PDF-1.7\nobject store upload")
	f.s3.put(f.root+"/object.pdf", s3Content, "application/pdf")
	s3Path := upload.S3Path(testBucket, f.root+"/object.pdf")

	data, err := f.storage.Open(ctx, fsPath)
	require.NoError(t, err)
	require.Equal(t, fsContent, data)
	body, info, err := f.storage.OpenReader(ctx, fsPath)
	require.NoError(t, err)
	require.Equal(t, fsContent, readAllAndClose(t, body))
	require.Equal(t, int64(len(fsContent)), info.Size)
	require.Equal(t, 0, f.s3.requestCount())

	data, err = f.storage.Open(ctx, s3Path)
	require.NoError(t, err)
	require.Equal(t, s3Content, data)
	body, info, err = f.storage.OpenReader(ctx, s3Path)
	require.NoError(t, err)
	require.Equal(t, s3Content, readAllAndClose(t, body))
	require.Equal(t, upload.ObjectInfo{Size: int64(len(s3Content)), ContentType: "application/pdf"}, info)
	_, err = os.Stat(filepath.Join(f.root, "object.pdf"))
	require.ErrorIs(t, err, fs.ErrNotExist)

	_, _, err = f.storage.OpenReader(ctx, upload.S3Path(testBucket, f.root+"/missing.pdf"))
	require.ErrorIs(t, err, fs.ErrNotExist)
	_, _, err = f.storage.OpenReader(ctx, upload.S3Path("other-bucket", f.root+"/object.pdf"))
	require.ErrorIs(t, err, upload.ErrInvalidS3Path)

	presigned, err := f.storage.PresignGetURL(ctx, s3Path, 0)
	require.NoError(t, err)
	require.Equal(t, "https://minio.test/uploads/"+f.root+"/object.pdf?X-Amz-Expires=300", presigned)
	_, err = f.storage.PresignGetURL(ctx, fsPath, time.Minute)
	require.ErrorIs(t, err, upload.ErrPresignNotSupported)

	renamedPath := upload.S3Path(testBucket, f.root+"/renamed object.pdf")
	require.NoError(t, f.storage.Rename(ctx, s3Path, renamedPath))
	_, ok := f.s3.object(f.root + "/object.pdf")
	require.False(t, ok)
	renamed, ok := f.s3.object(f.root + "/renamed object.pdf")
	require.True(t, ok)
	require.Equal(t, s3Content, renamed)

	require.NoError(t, f.storage.Delete(ctx, renamedPath))
	_, ok = f.s3.object(f.root + "/renamed object.pdf")
	require.False(t, ok)
	require.NoError(t, f.storage.Delete(ctx, fsPath))
	_, err = os.Stat(fsPath)
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestRoutedStorage_UnconfiguredS3ReturnsError(t *testing.T) {
	// Falsely green if a nil S3 backend panicked inside a recovered handler
	// instead of returning the sentinel error.
	root := t.TempDir()
	fsStorage, err := persistence.NewFSStorage(&uploadsconfig.Config{Path: root})
	require.NoError(t, err)
	routed, err := persistence.NewRoutedStorage(fsStorage, nil, false)
	require.NoError(t, err)
	ctx := context.Background()
	s3Path := upload.S3Path(testBucket, "static/a.pdf")

	_, err = routed.Open(ctx, s3Path)
	require.ErrorIs(t, err, upload.ErrS3NotConfigured)
	_, _, err = routed.OpenReader(ctx, s3Path)
	require.ErrorIs(t, err, upload.ErrS3NotConfigured)
	require.ErrorIs(t, routed.Save(ctx, s3Path, []byte("x")), upload.ErrS3NotConfigured)
	require.ErrorIs(t, routed.Delete(ctx, s3Path), upload.ErrS3NotConfigured)
	require.ErrorIs(t, routed.Rename(ctx, s3Path, upload.S3Path(testBucket, "static/b.pdf")), upload.ErrS3NotConfigured)
	_, err = routed.PresignGetURL(ctx, s3Path, time.Minute)
	require.ErrorIs(t, err, upload.ErrS3NotConfigured)
	require.ErrorIs(t, routed.Rename(ctx, filepath.Join(root, "a.pdf"), s3Path), upload.ErrStorageBackendMismatch)

	localPath := filepath.Join(root, "a.txt")
	require.NoError(t, routed.Save(ctx, localPath, []byte("local")))
	data, err := routed.Open(ctx, localPath)
	require.NoError(t, err)
	require.Equal(t, []byte("local"), data)

	_, err = persistence.NewRoutedStorage(fsStorage, nil, true)
	require.ErrorIs(t, err, upload.ErrS3NotConfigured)
}

func TestNewUploadStorage_SelectsPrimaryFromConfig(t *testing.T) {
	// Falsely green if S3 were only constructed for an S3 primary: an fs
	// primary with S3 settings must still build the S3 reader.
	root := t.TempDir()
	uploadsCfg := &uploadsconfig.Config{Path: root}
	s3Settings := uploadsconfig.S3Config{
		Endpoint: "minio.internal:9000",
		Bucket:   testBucket,
		Access:   uploadsconfig.S3CredentialPart{Key: "access"},
		Secret:   uploadsconfig.S3CredentialPart{Key: "secret"},
	}

	defaults, err := persistence.NewUploadStorage(uploadsCfg, &uploadsconfig.StorageConfig{})
	require.NoError(t, err)
	require.Empty(t, defaults.StorageLocation())
	_, err = defaults.Open(context.Background(), upload.S3Path(testBucket, "static/a.pdf"))
	require.ErrorIs(t, err, upload.ErrS3NotConfigured)

	fsPrimary, err := persistence.NewUploadStorage(uploadsCfg, &uploadsconfig.StorageConfig{
		Storage: uploadsconfig.StorageBackendConfig{Backend: uploadsconfig.BackendFS}, S3: s3Settings,
	})
	require.NoError(t, err)
	require.Empty(t, fsPrimary.StorageLocation())
	_, err = fsPrimary.Open(context.Background(), upload.S3Path("other-bucket", "static/a.pdf"))
	require.ErrorIs(t, err, upload.ErrInvalidS3Path)

	s3Primary, err := persistence.NewUploadStorage(uploadsCfg, &uploadsconfig.StorageConfig{
		Storage: uploadsconfig.StorageBackendConfig{Backend: uploadsconfig.BackendS3}, S3: s3Settings,
	})
	require.NoError(t, err)
	require.Equal(t, "s3://"+testBucket, s3Primary.StorageLocation())

	_, err = persistence.NewUploadStorage(uploadsCfg, &uploadsconfig.StorageConfig{
		Storage: uploadsconfig.StorageBackendConfig{Backend: uploadsconfig.BackendS3},
	})
	require.ErrorContains(t, err, "UPLOAD_S3_BUCKET")
}

func TestUploadService_WritesNewUploadsToPrimaryBackend(t *testing.T) {
	// Falsely green if both backends received the bytes: each case asserts
	// the other backend stayed empty.
	for _, s3Primary := range []bool{false, true} {
		t.Run(map[bool]string{false: "fs primary", true: "s3 primary"}[s3Primary], func(t *testing.T) {
			f := newDualStorageFixture(t, s3Primary)
			service := services.NewConfiguredUploadService(&uploadMetadata{}, f.storage, eventbus.NewEventPublisher(logrus.New()), f.cfg)
			content := []byte("%PDF-1.7\nnew upload")

			created, err := service.Create(context.Background(), &upload.CreateDTO{File: bytes.NewReader(content), Name: "policy.pdf", Size: len(content)})
			require.NoError(t, err)

			localPath := filepath.Join(f.root, created.Hash()+".pdf")
			key := filepath.ToSlash(localPath)
			stored, inS3 := f.s3.object(key)
			_, diskErr := os.Stat(localPath)
			if s3Primary {
				require.Equal(t, upload.S3Path(testBucket, key), created.Path())
				require.True(t, inS3)
				require.Equal(t, content, stored)
				require.ErrorIs(t, diskErr, fs.ErrNotExist)
			} else {
				require.Equal(t, localPath, created.Path())
				require.False(t, inS3)
				require.NoError(t, diskErr)
				require.Equal(t, 0, f.s3.requestCount())
			}

			body, _, err := service.OpenReader(context.Background(), created.Path())
			require.NoError(t, err)
			require.Equal(t, content, readAllAndClose(t, body))
			require.True(t, service.IsAccessible(context.Background(), created))
		})
	}
}

func TestUploadService_S3PrimaryKeepsPrivateNamespaceAndExistingFiles(t *testing.T) {
	// Falsely green if the private-path guard compared s3:// strings with
	// filesystem roots, or if deduplication copied legacy files into S3.
	f := newDualStorageFixture(t, true)
	publisher := eventbus.NewEventPublisher(logrus.New())
	content := []byte("%PDF-1.7\nprivate document")
	privatePath := filepath.Join(f.cfg.PrivatePath(), "edo")
	slug := "0123456789abcdef0123456789abcdef"

	private, err := services.NewConfiguredUploadService(&uploadMetadata{}, f.storage, publisher, f.cfg).CreatePrivate(
		context.Background(),
		&upload.CreateDTO{File: bytes.NewReader(content), Name: "edo.pdf", Size: len(content), Slug: slug},
		privatePath,
	)
	require.NoError(t, err)
	privateKey := filepath.ToSlash(filepath.Join(privatePath, slug+".pdf"))
	require.Equal(t, upload.S3Path(testBucket, privateKey), private.Path())
	_, ok := f.s3.object(privateKey)
	require.True(t, ok)

	legacyContent := []byte("%PDF-1.7\nlegacy document on disk")
	existing, _, err := (&upload.CreateDTO{File: bytes.NewReader(legacyContent), Name: "legacy.pdf", Size: len(legacyContent), UploadsPath: f.root}).ToEntity()
	require.NoError(t, err)
	existing.SetID(9)
	require.NoError(t, os.WriteFile(existing.Path(), legacyContent, 0o600))
	before := f.s3.requestCount()

	deduplicated, err := services.NewConfiguredUploadService(&uploadMetadata{existing: existing}, f.storage, publisher, f.cfg).Create(
		context.Background(),
		&upload.CreateDTO{File: bytes.NewReader(legacyContent), Name: "legacy.pdf", Size: len(legacyContent)},
	)
	require.NoError(t, err)
	require.Equal(t, existing.Path(), deduplicated.Path())
	require.Equal(t, before, f.s3.requestCount())
}

type uploadsByID struct {
	upload.Repository
	uploads map[uint]upload.Upload
}

func (r *uploadsByID) GetByID(_ context.Context, id uint) (upload.Upload, error) {
	if entity, ok := r.uploads[id]; ok {
		return entity, nil
	}
	return nil, persistence.ErrUploadNotFound
}

func newUploadEntity(t *testing.T, id uint, storedPath string, content []byte) upload.Upload {
	t.Helper()
	entity, _, err := (&upload.CreateDTO{File: bytes.NewReader(content), Name: filepath.Base(storedPath), Size: len(content)}).ToEntity()
	require.NoError(t, err)
	return upload.NewWithID(id, testTenantID, entity.Hash(), storedPath, entity.Name(), entity.Slug(), len(content), entity.Mimetype(), entity.Type(), time.Now(), time.Now())
}

func TestUploadController_PublicHandlerNeverServesS3OrPrivateUploads(t *testing.T) {
	// Falsely green if the objects were missing: the S3 object and the private
	// file both exist, while the public file next to them is served.
	f := newDualStorageFixture(t, true)
	publicContent := []byte("public file on disk")
	require.NoError(t, os.WriteFile(filepath.Join(f.root, "public.txt"), publicContent, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(f.cfg.PrivatePath(), "secret.txt"), []byte("private"), 0o600))
	f.s3.put(f.root+"/object.pdf", []byte("%PDF-1.7\ns3"), "application/pdf")
	f.s3.put(f.root+"/.private/object.pdf", []byte("%PDF-1.7\nprivate s3"), "application/pdf")

	service := services.NewConfiguredUploadService(&uploadsByID{uploads: map[uint]upload.Upload{}}, f.storage, eventbus.NewEventPublisher(logrus.New()), f.cfg)
	router := mux.NewRouter()
	controllers.NewUploadController(service, f.cfg).Register(router)
	server := httptest.NewServer(router)
	defer server.Close()

	get := func(requestPath string) (int, []byte) {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+requestPath, nil)
		require.NoError(t, err)
		response, err := server.Client().Do(request)
		require.NoError(t, err)
		defer func() { require.NoError(t, response.Body.Close()) }()
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		return response.StatusCode, body
	}

	status, body := get("/" + f.root + "/public.txt")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, publicContent, body)
	for _, requestPath := range []string{
		"/" + f.root + "/object.pdf",
		"/" + f.root + "/.private/secret.txt",
		"/" + f.root + "/.private/object.pdf",
		"/" + f.root + "/s3:/" + testBucket + "/" + f.root + "/object.pdf",
	} {
		status, _ := get(requestPath)
		require.Equal(t, http.StatusNotFound, status, requestPath)
	}
}

func TestUploadController_DownloadStreamsS3UploadsToSignedInUsers(t *testing.T) {
	// Falsely green if the route ignored the session: the same request is
	// rejected without a user and accepted with one.
	f := newDualStorageFixture(t, true)
	content := []byte("%PDF-1.7\ns3 download")
	publicKey := f.root + "/download.pdf"
	f.s3.put(publicKey, content, "application/pdf")
	f.s3.put(f.root+"/.private/edo.pdf", content, "application/pdf")
	f.s3.put("elsewhere/edo.pdf", content, "application/pdf")
	foreignKey := f.root + "/foreign.pdf"
	f.s3.put(foreignKey, []byte("%PDF-1.7\nforeign tenant"), "application/pdf")
	own := newUploadEntity(t, 5, upload.S3Path(testBucket, foreignKey), content)
	foreign := upload.NewWithID(5, uuid.MustParse("00000000-0000-0000-0000-000000000002"), own.Hash(), own.Path(), own.Name(), own.Slug(), len(content), own.Mimetype(), own.Type(), time.Now(), time.Now())
	repo := &uploadsByID{uploads: map[uint]upload.Upload{
		1: newUploadEntity(t, 1, upload.S3Path(testBucket, publicKey), content),
		2: newUploadEntity(t, 2, upload.S3Path(testBucket, f.root+"/.private/edo.pdf"), content),
		3: newUploadEntity(t, 3, upload.S3Path(testBucket, "elsewhere/edo.pdf"), content),
		4: newUploadEntity(t, 4, upload.S3Path(testBucket, f.root+"/missing.pdf"), content),
		5: foreign,
	}}
	require.Equal(t, "/uploads/1", repo.uploads[1].PreviewURL())

	service := services.NewConfiguredUploadService(repo, f.storage, eventbus.NewEventPublisher(logrus.New()), f.cfg)
	router := mux.NewRouter()
	controllers.NewUploadController(service, f.cfg).Register(router)
	email, err := internet.NewEmail("viewer@example.com")
	require.NoError(t, err)
	viewer := user.New("View", "Er", email, user.UILanguageEN)

	serve := func(requestPath string, signedIn bool) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, requestPath, nil)
		if signedIn {
			request = request.WithContext(composables.WithTenantID(composables.WithUser(request.Context(), viewer), testTenantID))
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		return recorder
	}

	require.Equal(t, http.StatusUnauthorized, serve("/uploads/1", false).Code)

	response := serve("/uploads/1", true)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, content, response.Body.Bytes())
	require.Equal(t, "application/pdf", response.Header().Get("Content-Type"))
	require.Equal(t, fmt.Sprint(len(content)), response.Header().Get("Content-Length"))
	require.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "sandbox", response.Header().Get("Content-Security-Policy"))

	for _, requestPath := range []string{"/uploads/2", "/uploads/3", "/uploads/4", "/uploads/5", "/uploads/99"} {
		missing := serve(requestPath, true)
		require.Equal(t, http.StatusNotFound, missing.Code, requestPath)
		require.NotContains(t, missing.Body.String(), "foreign tenant", requestPath)
	}

	withoutTenant := httptest.NewRequest(http.MethodGet, "/uploads/1", nil)
	withoutTenant = withoutTenant.WithContext(composables.WithUser(withoutTenant.Context(), viewer))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, withoutTenant)
	require.Equal(t, http.StatusNotFound, recorder.Code)

	// The MIME type detected at upload wins over what the store reports.
	f.s3.put(publicKey, content, "text/html")
	require.Equal(t, "application/pdf", serve("/uploads/1", true).Header().Get("Content-Type"))
}

// noSuchBucketError carries both the error code and the HTTP 404 of a real
// response, so a check on the status alone would call it a missing object.
type noSuchBucketError struct{}

func (noSuchBucketError) Error() string       { return "NoSuchBucket" }
func (noSuchBucketError) ErrorCode() string   { return "NoSuchBucket" }
func (noSuchBucketError) HTTPStatusCode() int { return http.StatusNotFound }

// missingBucketS3 answers every object call the way S3 does for an absent bucket.
type missingBucketS3 struct{ *memoryS3 }

func (missingBucketS3) GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return nil, noSuchBucketError{}
}

func (missingBucketS3) DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return nil, noSuchBucketError{}
}

func TestS3Storage_MissingBucketIsNotAMissingFile(t *testing.T) {
	client := missingBucketS3{newMemoryS3()}
	storage := persistence.NewS3StorageWithClient(testBucket, client, client)
	path := upload.S3Path(testBucket, "static/a.pdf")

	_, _, err := storage.OpenReader(t.Context(), path)
	require.Error(t, err)
	require.NotErrorIs(t, err, fs.ErrNotExist)
	require.Error(t, storage.Delete(t.Context(), path))

	present := newMemoryS3()
	_, _, err = persistence.NewS3StorageWithClient(testBucket, present, present).OpenReader(t.Context(), path)
	require.ErrorIs(t, err, fs.ErrNotExist)
}

// swapRepo holds the two rows of a slug swap and can fail the Nth Update.
type swapRepo struct {
	upload.Repository
	bySlug, byHash map[string]upload.Upload
	failUpdate     int
	updates        int
}

func (r *swapRepo) GetBySlug(_ context.Context, slug string) (upload.Upload, error) {
	if entity, ok := r.bySlug[slug]; ok {
		return entity, nil
	}
	return nil, persistence.ErrUploadNotFound
}

func (r *swapRepo) GetByHash(_ context.Context, hash string) (upload.Upload, error) {
	if entity, ok := r.byHash[hash]; ok {
		return entity, nil
	}
	return nil, persistence.ErrUploadNotFound
}

func (r *swapRepo) Update(context.Context, upload.Upload) error {
	r.updates++
	if r.updates == r.failUpdate {
		return errors.New("row update failed")
	}
	return nil
}

// failingRename fails the Nth Rename and passes everything else through.
type failingRename struct {
	*persistence.RoutedStorage
	failAt, calls int
}

func (s *failingRename) Rename(ctx context.Context, oldPath, newPath string) error {
	s.calls++
	if s.calls == s.failAt {
		return errors.New("rename failed")
	}
	return s.RoutedStorage.Rename(ctx, oldPath, newPath)
}

func TestUploadService_FailedSlugMoveKeepsObjectsWhereRowsPoint(t *testing.T) {
	holderBytes := []byte("%PDF-1.7\nslug holder")
	movedBytes := []byte("%PDF-1.7\nexisting content")
	tests := []struct {
		name          string
		failRename    int
		failUpdate    int
		wantRowWrites int
	}{
		{name: "second rename fails", failRename: 2, wantRowWrites: 0},
		{name: "first row update fails", failUpdate: 1, wantRowWrites: 1},
		{name: "second row update fails", failUpdate: 2, wantRowWrites: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newDualStorageFixture(t, true)
			holderKey := f.root + "/report.pdf"
			f.s3.put(holderKey, holderBytes, "application/pdf")
			holderEntity := newUploadEntity(t, 1, upload.S3Path(testBucket, holderKey), holderBytes)
			holderEntity.SetSlug("report")
			require.Equal(t, upload.S3Path(testBucket, holderKey), holderEntity.Path())

			moved := newUploadEntity(t, 2, upload.S3Path(testBucket, f.root+"/x.pdf"), movedBytes)
			moved.SetSlug(moved.Hash())
			movedKey := f.root + "/" + moved.Hash() + ".pdf"
			f.s3.put(movedKey, movedBytes, "application/pdf")

			repo := &swapRepo{
				bySlug:     map[string]upload.Upload{"report": holderEntity, moved.Hash(): moved},
				byHash:     map[string]upload.Upload{holderEntity.Hash(): holderEntity, moved.Hash(): moved},
				failUpdate: tt.failUpdate,
			}
			storage := &failingRename{RoutedStorage: f.storage, failAt: tt.failRename}
			service := services.NewConfiguredUploadService(repo, storage, eventbus.NewEventPublisher(logrus.New()), f.cfg)

			_, err := service.Create(t.Context(), &upload.CreateDTO{File: bytes.NewReader(movedBytes), Name: "report.pdf", Slug: "report", Size: len(movedBytes)})
			require.Error(t, err)

			require.Equal(t, tt.wantRowWrites, repo.updates)
			require.Equal(t, "report", holderEntity.Slug())
			require.Equal(t, moved.Hash(), moved.Slug())
			got, ok := f.s3.object(holderKey)
			require.True(t, ok)
			require.Equal(t, holderBytes, got)
			got, ok = f.s3.object(movedKey)
			require.True(t, ok)
			require.Equal(t, movedBytes, got)
			_, ok = f.s3.object(f.root + "/" + holderEntity.Hash() + ".pdf")
			require.False(t, ok, "parked copy must be undone")
		})
	}
}
