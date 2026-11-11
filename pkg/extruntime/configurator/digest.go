package configurator

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
)

type digest struct{ h hash.Hash }

func newDigest() *digest { return &digest{h: sha256.New()} }

func (d *digest) add(path, content string) {
	_, _ = d.h.Write([]byte(path))
	_, _ = d.h.Write([]byte{0})
	_, _ = d.h.Write([]byte(content))
	_, _ = d.h.Write([]byte{0})
}

func (d *digest) sum() string { return hex.EncodeToString(d.h.Sum(nil)) }
