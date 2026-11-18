package build

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
)

// HashTree is the content hash used for Result.SourceHash and
// Result.ArtifactHash: sha256 over the files in sorted path order, each as
// path NUL decimal-length NUL content.
func HashTree(t Tree) string {
	paths := make([]string, 0, len(t))
	for p := range t {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write([]byte(strconv.Itoa(len(t[p]))))
		h.Write([]byte{0})
		h.Write(t[p])
	}
	return hex.EncodeToString(h.Sum(nil))
}
