package spotlight

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/iota-uz/go-i18n/v2/i18n"
)

//go:embed locales/*.json
var defaultLocaleFiles embed.FS

func LocaleFS() *embed.FS {
	return &defaultLocaleFiles
}

func RegisterTranslations(bundle *i18n.Bundle) error {
	if bundle == nil {
		return fmt.Errorf("spotlight: bundle is nil")
	}
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	return fs.WalkDir(defaultLocaleFiles, "locales", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := defaultLocaleFiles.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = bundle.ParseMessageFileBytes(data, filepath.Base(path))
		return err
	})
}

func MustRegisterTranslations(bundle *i18n.Bundle) {
	if err := RegisterTranslations(bundle); err != nil {
		panic(err)
	}
}
