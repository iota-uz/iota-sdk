package registry

import (
	"io/fs"

	lensspec "github.com/iota-uz/iota-sdk/pkg/lens/spec"
)

func LoadFS(fsys fs.FS, name string) (lensspec.Document, error) {
	return lensspec.LoadFS(fsys, name)
}

func MustLoadFS(fsys fs.FS, name string) lensspec.Document {
	doc, err := LoadFS(fsys, name)
	if err != nil {
		panic(err)
	}
	return doc
}
