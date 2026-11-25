package secrets

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
