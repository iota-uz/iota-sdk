//go:build !unix

package artifact

import "os"

// securePrivateDir restricts dir to the current user where the platform has
// no ownership check.
func securePrivateDir(dir string) error { return os.Chmod(dir, 0o700) }
