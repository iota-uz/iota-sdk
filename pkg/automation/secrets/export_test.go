package secrets

import "time"

// SetPublishHook makes every publishing statement of the manager return err after it ran (a COMMIT whose
// reply is lost); nil restores it.
func SetPublishHook(err error) func() {
	if err == nil {
		publishHook = nil
		return func() {}
	}
	publishHook = func() error { return err }
	return func() { publishHook = nil }
}

// SetWriteWindow shortens the write window (and its fence, half of it) for a test; the returned func
// restores them.
func SetWriteWindow(d time.Duration) func() {
	w, f := writeWindow, writeFence
	writeWindow, writeFence = d, d/2
	return func() { writeWindow, writeFence = w, f }
}
