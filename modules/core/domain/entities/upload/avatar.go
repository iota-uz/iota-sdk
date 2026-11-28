package upload

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

const AvatarAccept = "image/jpeg,image/png,image/apng,image/gif,image/webp,image/avif,image/bmp,image/x-icon"

var ErrAvatarType = errors.New("unsupported profile image type")

// ValidateAvatarType checks both the detected content and the suffix used to serve it.
func ValidateAvatarType(name string, detected *mimetype.MIME) error {
	ext := strings.ToLower(filepath.Ext(name))
	var want string
	switch ext {
	case ".jpg", ".jpeg", ".jpe", ".jfif":
		want = "image/jpeg"
	case ".png", ".apng":
		if detected != nil && detected.Is("image/vnd.mozilla.apng") {
			return nil
		}
		want = "image/png"
	case ".gif":
		want = "image/gif"
	case ".webp":
		want = "image/webp"
	case ".avif":
		want = "image/avif"
	case ".bmp":
		want = "image/bmp"
	case ".ico":
		want = "image/x-icon"
	default:
		return ErrAvatarType
	}
	if detected == nil || !detected.Is(want) {
		return ErrAvatarType
	}
	return nil
}
