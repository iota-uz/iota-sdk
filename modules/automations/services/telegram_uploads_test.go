package services_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	tgconn "github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

type coreUploads struct {
	items map[uint]upload.Upload
	next  uint
}

func (c *coreUploads) Create(ctx context.Context, d *upload.CreateDTO) (upload.Upload, error) {
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	c.next++
	data, _ := io.ReadAll(d.File)
	u := upload.NewWithID(c.next, tenant, "h"+d.Name, "/uploads/"+d.Name, d.Name, "", len(data), mimetype.Detect(data), upload.UploadTypeDocument, time.Now(), time.Now())
	if c.items == nil {
		c.items = map[uint]upload.Upload{}
	}
	c.items[c.next] = u
	return u, nil
}

func (c *coreUploads) GetByID(_ context.Context, id uint) (upload.Upload, error) {
	if u, ok := c.items[id]; ok {
		return u, nil
	}
	return nil, io.EOF
}

func (c *coreUploads) OpenReader(context.Context, string) (io.ReadCloser, upload.ObjectInfo, error) {
	return io.NopCloser(strings.NewReader("contents")), upload.ObjectInfo{}, nil
}

func TestTelegramUploadsAreTheTenants(t *testing.T) {
	core := &coreUploads{}
	up := services.NewTelegramUploads(core)
	tenant := uuid.New()
	saved, err := up.Save(context.Background(), tenant, tgconn.UploadFile{Name: "a.txt", Data: []byte("hello")})
	require.NoError(t, err)
	assert.Equal(t, "1", saved.ID)
	assert.NotEmpty(t, saved.URL)
	f, err := up.Open(context.Background(), tenant, saved.ID)
	require.NoError(t, err)
	assert.Equal(t, "a.txt", f.Name)
	assert.Equal(t, []byte("contents"), f.Data)
	_, err = up.Open(context.Background(), uuid.New(), saved.ID)
	assert.Error(t, err, "another tenant's upload is not found")
	_, err = up.Open(context.Background(), tenant, "nope")
	assert.Error(t, err)
}
