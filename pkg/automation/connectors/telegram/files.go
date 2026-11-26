package telegram

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

// Limits of files: what the Bot API lets a bot download, and upload.
const (
	maxDownload = 20 << 20
	maxUpload   = 50 << 20
)

// Uploads is the uploads storage of the SDK, as the connector needs it: the files a bot received are
// stored in it, and the files a node sends are read from it. The application provides the adapter over its
// upload service; every call is for one tenant.
type Uploads interface {
	// Save stores a file; the id is what a graph passes to telegram.send, the URL is where a person opens it.
	Save(ctx context.Context, tenantID uuid.UUID, f UploadFile) (Stored, error)
	// Open reads a stored file of the tenant; an id that is not the tenant's is an error.
	Open(ctx context.Context, tenantID uuid.UUID, id string) (UploadFile, error)
}

// UploadFile is a file of the uploads storage.
type UploadFile struct {
	Name string
	Mime string
	Data []byte
}

// Stored is a saved file.
type Stored struct {
	ID  string
	URL string
}

// Codes of files[].error: why an attachment of an incoming message was not stored.
const (
	FileTooLarge     = "too_large"
	FileDownload     = "download_failed"
	FileStorage      = "storage_failed"
	FileNotConfigure = "no_storage"
)

// attachments reads the uploads a send names.
func (c *Connector) attachments(ctx context.Context, tenant uuid.UUID, v any) ([]file, error) {
	list, _ := v.([]any)
	if len(list) == 0 {
		return nil, nil
	}
	if c.cfg.Uploads == nil {
		return nil, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "files are not available: the application has no uploads storage for the connector"}
	}
	out := make([]file, 0, len(list))
	for _, item := range list {
		id := fmt.Sprint(item)
		if n, ok := intOf(item); ok {
			id = strconv.FormatInt(n, 10)
		}
		f, err := c.cfg.Uploads.Open(ctx, tenant, id)
		switch {
		case err != nil && ctx.Err() != nil:
			return nil, ctx.Err()
		case err != nil:
			return nil, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "the file " + id + " is not available"}
		case len(f.Data) > maxUpload:
			return nil, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "the file " + id + " is larger than 50 MB, what a bot may send"}
		}
		name := f.Name
		if name == "" {
			name = "file-" + id
		}
		out = append(out, file{Name: name, Mime: f.Mime, Data: f.Data})
	}
	return out, nil
}

// incomingFile is an attachment of an incoming message as the event carries it.
type incomingFile struct {
	UploadID string `json:"upload_id,omitempty"`
	Name     string `json:"name"`
	Mime     string `json:"mime,omitempty"`
	Size     int64  `json:"size"`
	URL      string `json:"url,omitempty"`
	Error    string `json:"error,omitempty"`
}

// fileRef is what the message names of a file before it is downloaded.
type fileRef struct {
	id, name, mime string
	size           int64
}

// refsOf is the photo (its largest size) and the document of a message: the files that are stored.
func refsOf(m *gotgbot.Message) []fileRef {
	var out []fileRef
	if n := len(m.Photo); n > 0 {
		p := m.Photo[n-1]
		out = append(out, fileRef{id: p.FileId, name: "photo-" + strconv.FormatInt(m.MessageId, 10) + ".jpg", mime: "image/jpeg", size: p.FileSize})
	}
	if d := m.Document; d != nil {
		name := d.FileName
		if name == "" {
			name = "document-" + strconv.FormatInt(m.MessageId, 10)
		}
		mime := d.MimeType
		if mime == "" {
			mime = "application/octet-stream"
		}
		out = append(out, fileRef{id: d.FileId, name: name, mime: mime, size: d.FileSize})
	}
	return out
}

// store downloads the files of a message and saves them in the uploads storage, as the message arrives. A
// file that cannot be stored is still in the list, with the reason: the event is never lost for it.
func (c *Connector) store(ctx context.Context, b *bot, tenant uuid.UUID, dedup string, refs []fileRef) []incomingFile {
	out := make([]incomingFile, 0, len(refs))
	for _, r := range refs {
		f := incomingFile{Name: r.name, Mime: r.mime, Size: r.size}
		switch {
		case c.cfg.Uploads == nil:
			f.Error = FileNotConfigure
		case r.size > maxDownload:
			f.Error = FileTooLarge
		default:
			// A replay of the update (Telegram did not get the answer) finds the file saved by the delivery before
			// it, so the event the engine keeps references the one upload and no second copy is left behind.
			memo := tenant.String() + "/" + dedup + "/" + r.id
			if saved, ok := c.uploaded.get(memo); ok {
				f = saved
			} else {
				c.download(ctx, b, tenant, r, &f)
				if f.UploadID != "" {
					c.uploaded.put(memo, f)
				}
			}
		}
		out = append(out, f)
	}
	return out
}

func (c *Connector) download(ctx context.Context, b *bot, tenant uuid.UUID, r fileRef, f *incomingFile) {
	meta, err := b.getFile(ctx, r.id)
	if err != nil {
		f.Error = FileDownload
		var api *apiError
		if errors.As(err, &api) && api.Code == 400 && strings.Contains(strings.ToLower(api.Description), "file is too big") {
			f.Error = FileTooLarge // getFile refuses files over 20 MB
		}
		return
	}
	data, err := b.download(ctx, meta, maxDownload)
	switch {
	case errors.Is(err, errTooLarge):
		f.Error = FileTooLarge
		return
	case err != nil:
		f.Error = FileDownload
		return
	}
	saved, err := c.cfg.Uploads.Save(ctx, tenant, UploadFile{Name: r.name, Mime: r.mime, Data: data})
	if err != nil {
		f.Error = FileStorage
		return
	}
	f.UploadID, f.URL, f.Size = saved.ID, saved.URL, int64(len(data))
}

// savedUploads remembers the uploads of the updates handled lately (this process only): a replay of an update
// reuses them instead of saving the files again. An entry lives for uploadMemoTTL; a replay after that, or by
// another replica, saves a copy, which nothing references.
type savedUploads struct {
	mu sync.Mutex
	m  map[string]memoFile
}

type memoFile struct {
	file    incomingFile
	expires time.Time
}

const (
	uploadMemoTTL = 15 * time.Minute
	uploadMemoMax = 1000
)

func (s *savedUploads) get(key string) (incomingFile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[key]
	if !ok || time.Now().After(e.expires) {
		return incomingFile{}, false
	}
	return e.file, true
}

func (s *savedUploads) put(key string, f incomingFile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if s.m == nil || len(s.m) >= uploadMemoMax {
		for k, e := range s.m {
			if now.After(e.expires) {
				delete(s.m, k)
			}
		}
		if len(s.m) >= uploadMemoMax {
			s.m = map[string]memoFile{}
		}
		if s.m == nil {
			s.m = map[string]memoFile{}
		}
	}
	s.m[key] = memoFile{file: f, expires: now.Add(uploadMemoTTL)}
}
