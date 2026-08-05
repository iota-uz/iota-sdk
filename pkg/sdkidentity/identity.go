// Package sdkidentity defines the release identity shared by the Go module and
// the canonical @iota-uz/sdk JavaScript distribution.
package sdkidentity

import (
	"fmt"
	"strings"
)

const (
	ReleaseVersion  = "0.5.0"
	ProtocolVersion = "1.0.0"
)

type Identity struct {
	ReleaseVersion  string `json:"releaseVersion"`
	SourceCommit    string `json:"sourceCommit"`
	ProtocolVersion string `json:"protocolVersion"`
}

func (identity Identity) Validate() error {
	if identity.ReleaseVersion != ReleaseVersion {
		return fmt.Errorf("sdk identity: release %q is incompatible with %q", identity.ReleaseVersion, ReleaseVersion)
	}
	if len(identity.SourceCommit) != 40 {
		return fmt.Errorf("sdk identity: source commit must be a full Git SHA")
	}
	for _, character := range identity.SourceCommit {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return fmt.Errorf("sdk identity: source commit must be lowercase hexadecimal")
		}
	}
	if protocolMajor(identity.ProtocolVersion) != protocolMajor(ProtocolVersion) {
		return fmt.Errorf("sdk identity: protocol %q is incompatible with %q", identity.ProtocolVersion, ProtocolVersion)
	}
	return nil
}

func protocolMajor(version string) string {
	return strings.SplitN(strings.TrimSpace(version), ".", 2)[0]
}
