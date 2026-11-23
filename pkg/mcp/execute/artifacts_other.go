//go:build !unix

package execute

import "context"

// collectArtifacts returns nothing where descriptor-relative traversal is not
// available: the host never follows paths the script controls.
func (r *Runner) collectArtifacts(context.Context, string) []Artifact { return []Artifact{} }
