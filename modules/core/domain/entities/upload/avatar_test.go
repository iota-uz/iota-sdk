package upload_test

import (
	"testing"

	"github.com/gabriel-vasile/mimetype"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/stretchr/testify/require"
)

func TestValidateAvatarType(t *testing.T) {
	for _, tc := range []struct {
		name  string
		mime  string
		valid bool
	}{
		{"portrait.JPG", "image/jpeg", true},
		{"portrait.jpeg", "image/jpeg", true},
		{"portrait.png", "image/png", true},
		{"animated.png", "image/vnd.mozilla.apng", true},
		{"portrait.gif", "image/gif", true},
		{"portrait.webp", "image/webp", true},
		{"portrait.avif", "image/avif", true},
		{"portrait.bmp", "image/bmp", true},
		{"portrait.ico", "image/x-icon", true},
		{"portrait.png", "text/plain", false},
		{"portrait.txt", "image/png", false},
		{"portrait.svg", "image/svg+xml", false},
		{"portrait", "image/jpeg", false},
		{"portrait.png", "", false},
	} {
		t.Run(tc.name+"_"+tc.mime, func(t *testing.T) {
			detected := mimetype.Lookup(tc.mime)
			if tc.mime != "" {
				require.NotNil(t, detected)
			}
			err := upload.ValidateAvatarType(tc.name, detected)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, upload.ErrAvatarType)
			}
		})
	}
}
