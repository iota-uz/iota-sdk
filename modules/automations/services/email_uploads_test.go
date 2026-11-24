package services_test

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

// The adapter runs over the real upload service, repository and storage, from a context that carries
// neither a pool nor a tenant — what a poll has.
func TestEmailUploadsOverTheSDKStorage(t *testing.T) {
	e := newEnv(t)
	cfg := &uploadsconfig.Config{Path: t.TempDir()}
	storage, err := persistence.NewFSStorage(cfg)
	require.NoError(t, err)
	core := coreservices.NewConfiguredUploadService(persistence.NewUploadRepository(), storage, eventbus.NewEventPublisher(logrus.New()), cfg)
	up := services.NewEmailUploads(core, e.db)

	saved, err := up.Save(context.Background(), e.tenant, email.UploadFile{Name: "cat.png", Mime: "image/png", Data: []byte("\x89PNG\r\n\x1a\n image bytes")})
	require.NoError(t, err, "an attachment is stored with no scope in the context")
	assert.NotEmpty(t, saved.ID)
	assert.NotEmpty(t, saved.URL, "the event carries where the file is opened")
}
