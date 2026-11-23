//go:build !linux

package isolation

import "errors"

func verifyActFor(int, *Credential) error { return errors.New("requires Linux") }
