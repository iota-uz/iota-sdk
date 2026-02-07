package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
)

func renderChartArtifact(artifact domain.Artifact, mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "default"
	}

	if mode == "visual" {
		return "## Chart Spec\n\nChart visual mode is not implemented yet. Use mode=\"spec\"."
	}

	spec, ok := artifact.Metadata()["spec"]
	if !ok {
		return "## Chart Spec\n\n```json\n{}\n```"
	}

	pretty, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Sprintf("## Chart Spec\n\nFailed to render chart spec: %v", err)
	}

	return "## Chart Spec\n```json\n" + string(pretty) + "\n```"
}
