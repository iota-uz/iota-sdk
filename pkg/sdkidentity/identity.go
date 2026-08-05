// Package sdkidentity defines the release identity shared by the Go module and
// the canonical @iota-uz/sdk JavaScript distribution.
package sdkidentity

import (
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
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
	const op = serrors.Op("sdkidentity.Identity.Validate")

	if identity.ReleaseVersion != ReleaseVersion {
		return serrors.E(op, fmt.Errorf("release %q is incompatible with %q", identity.ReleaseVersion, ReleaseVersion))
	}
	if len(identity.SourceCommit) != 40 {
		return serrors.E(op, fmt.Errorf("source commit must be a full Git SHA"))
	}
	for _, character := range identity.SourceCommit {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return serrors.E(op, fmt.Errorf("source commit must be lowercase hexadecimal"))
		}
	}
	if protocolMajor(identity.ProtocolVersion) != protocolMajor(ProtocolVersion) {
		return serrors.E(op, fmt.Errorf("protocol %q is incompatible with %q", identity.ProtocolVersion, ProtocolVersion))
	}
	return nil
}

func protocolMajor(version string) string {
	return strings.SplitN(strings.TrimSpace(version), ".", 2)[0]
}
