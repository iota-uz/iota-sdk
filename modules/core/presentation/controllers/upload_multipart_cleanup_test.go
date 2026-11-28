package controllers_test

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

type spyUploadsAuthorizer struct {
	types.UploadsAuthorizer
	onCheck func(ctx context.Context) error
}

func (s spyUploadsAuthorizer) CanUploadFile(ctx context.Context) error {
	return s.onCheck(ctx)
}

func (s spyUploadsAuthorizer) CanUploadFileWithSlug(ctx context.Context) error {
	return s.onCheck(ctx)
}

func newSpillingMultipartRequest(t *testing.T, target, fieldName, filename string, payload []byte, truncate bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, filename)
	require.NoError(t, err)
	_, err = part.Write(payload)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	raw := body.Bytes()
	if truncate {
		// Drop the closing multipart boundary so ParseMultipartForm spills the
		// part to disk and then returns io.ErrUnexpectedEOF.
		raw = raw[:len(raw)-len(writer.Boundary())-6]
	}

	req := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(raw))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func multipartTempFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "multipart-") {
			out = append(out, entry.Name())
		}
	}
	return out
}

// Falsely green if ParseMultipartForm kept the upload in memory instead of
// spilling a temp file into TMPDIR, or if RemoveAll did not run on return.
func TestUploadController_Create_RemovesSpilledMultipartTempFiles(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TMPDIR", tempDir)

	cfg := &uploadsconfig.Config{
		Path:      "test-uploads",
		MaxMemory: 1, // Force every file larger than 1 byte to spill to disk.
		MaxSize:   1 << 20,
	}
	ctrl := controllers.NewUploadController(nil, cfg).(*controllers.UploadController)
	payload := bytes.Repeat([]byte("a"), 8*1024)

	t.Run("early return when file field missing cleans up spilled temp file", func(t *testing.T) {
		// First prove the same payload actually spills when ParseMultipartForm runs without cleanup.
		probeReq := newSpillingMultipartRequest(t, "/uploads", "other", "spill.bin", payload, false)
		require.NoError(t, probeReq.ParseMultipartForm(cfg.MaxMemory))
		require.NotEmpty(t, multipartTempFiles(t, tempDir), "expected ParseMultipartForm to spill a multipart temp file into TMPDIR")
		require.NoError(t, probeReq.MultipartForm.RemoveAll())
		require.Empty(t, multipartTempFiles(t, tempDir))

		req := newSpillingMultipartRequest(t, "/uploads", "other", "spill.bin", payload, false)
		rec := httptest.NewRecorder()
		ctrl.Create(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.NotNil(t, req.MultipartForm)
		require.Empty(t, multipartTempFiles(t, tempDir), "UploadController.Create must remove spilled multipart temp files")
	})

	t.Run("parse error after spilling leaves no temp files", func(t *testing.T) {
		req := newSpillingMultipartRequest(t, "/uploads", "file", "truncated.bin", payload, true)
		rec := httptest.NewRecorder()
		ctrl.Create(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Empty(t, multipartTempFiles(t, tempDir), "UploadController.Create must leave no temp files after parse error")
	})
}

// Falsely green if no temp file was observed in TMPDIR during handler execution,
// or if RemoveAll failed to delete it before returning.
func TestUploadAPIController_Create_RemovesSpilledMultipartTempFiles(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TMPDIR", tempDir)

	cfg := &uploadsconfig.Config{
		Path:      "test-uploads",
		MaxMemory: 1, // Force spill to TMPDIR.
		MaxSize:   1 << 20,
	}
	payload := bytes.Repeat([]byte("b"), 8*1024)

	var spilledDuringHandler int
	ctrl := controllers.NewUploadAPIController(
		nil,
		cfg,
		controllers.WithAPIUploadsAuthorizer(spyUploadsAuthorizer{
			onCheck: func(context.Context) error {
				spilledDuringHandler = len(multipartTempFiles(t, tempDir))
				return errors.New("forbidden in test")
			},
		}),
	).(*controllers.UploadAPIController)

	req := newSpillingMultipartRequest(t, "/api/uploads", "file", "spill.bin", payload, false)
	rec := httptest.NewRecorder()
	ctrl.Create(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Greater(t, spilledDuringHandler, 0, "multipart upload must spill a temp file into TMPDIR during handler execution")
	require.Empty(t, multipartTempFiles(t, tempDir), "UploadAPIController.Create must remove spilled multipart temp files on return")
}
