//go:build !linux

package isolation

import "errors"

func verifyActFor(int, *Credential) error { return errors.New("requires Linux") }

// KillMember is unavailable outside Linux.
func KillMember(int, int) bool { return false }
