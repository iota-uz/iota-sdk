package explore

import (
	"fmt"
	"strings"
)

type ExportMode string

const (
	ExportCurrentView ExportMode = "current_view"
	ExportFull        ExportMode = "full_exploration"
)

// ExportRequest identifies either the active explorer view or the complete
// multi-perspective exploration. Labels are resolved presentation values used
// only for workbook metadata and filenames; identity always comes from keys.
type ExportRequest struct {
	Mode           ExportMode   `json:"mode"`
	ExplorerID     string       `json:"explorerId"`
	BranchKey      string       `json:"branchKey"`
	PerspectiveKey string       `json:"perspectiveKey,omitempty"`
	Path           []string     `json:"path,omitempty"`
	Steps          []PathStep   `json:"steps,omitempty"`
	NodeKey        string       `json:"nodeKey,omitempty"`
	Labels         ExportLabels `json:"labels,omitempty"`
}

type ExportLabels struct {
	Explorer    string `json:"explorer,omitempty"`
	Branch      string `json:"branch,omitempty"`
	Perspective string `json:"perspective,omitempty"`
	Node        string `json:"node,omitempty"`
}

func (r ExportRequest) Validate() error {
	switch r.Mode {
	case ExportCurrentView, ExportFull:
	default:
		return fmt.Errorf("unsupported exploration export mode %q", r.Mode)
	}
	if err := validKey("explorer id", r.ExplorerID); err != nil {
		return err
	}
	if err := validKey("branch key", r.BranchKey); err != nil {
		return err
	}
	if r.Mode == ExportCurrentView {
		if err := validKey("perspective key", r.PerspectiveKey); err != nil {
			return err
		}
		steps := exportPathSteps(r)
		if len(steps) == 0 {
			return fmt.Errorf("current-view exploration export requires a path")
		}
		if err := validKey("node key", r.NodeKey); err != nil {
			return err
		}
		if steps[len(steps)-1].NodeKey != r.NodeKey {
			return fmt.Errorf("exploration export node %q must match final path node %q", r.NodeKey, steps[len(steps)-1].NodeKey)
		}
	}
	for _, step := range exportPathSteps(r) {
		if strings.TrimSpace(step.NodeKey) == "" {
			return fmt.Errorf("exploration export path contains a blank node")
		}
		if step.PointKey != strings.TrimSpace(step.PointKey) {
			return fmt.Errorf("exploration export path point %q has surrounding whitespace", step.PointKey)
		}
	}
	return nil
}

func ResolveExportRequest(spec Spec, request ExportRequest) (ExportRequest, error) {
	if err := spec.Validate(); err != nil {
		return ExportRequest{}, err
	}
	if err := request.Validate(); err != nil {
		return ExportRequest{}, err
	}
	if request.ExplorerID != spec.ID {
		return ExportRequest{}, fmt.Errorf("export request belongs to explorer %q, not %q", request.ExplorerID, spec.ID)
	}
	branch, ok := spec.Branch(request.BranchKey)
	if !ok {
		return ExportRequest{}, fmt.Errorf("explorer %s has no branch %q", spec.ID, request.BranchKey)
	}
	request.Labels.Explorer = spec.ID
	request.Labels.Branch = branch.Label
	if request.Mode == ExportFull {
		return request, nil
	}
	perspective, ok := branch.Perspective(request.PerspectiveKey)
	if !ok {
		return ExportRequest{}, fmt.Errorf("explorer %s branch %s has no perspective %q", spec.ID, branch.Key, request.PerspectiveKey)
	}
	steps := exportPathSteps(request)
	if steps[0].NodeKey != perspective.RootNode {
		return ExportRequest{}, fmt.Errorf("exploration export path must start at root node %q", perspective.RootNode)
	}
	for index, step := range steps {
		node, ok := perspective.Node(step.NodeKey)
		if !ok {
			return ExportRequest{}, fmt.Errorf("explorer %s perspective %s has no node %q", spec.ID, perspective.Key, step.NodeKey)
		}
		if index+1 < len(steps) && !nodeHasTarget(node, steps[index+1]) {
			return ExportRequest{}, fmt.Errorf("exploration export path cannot move from %q to %q", step.NodeKey, steps[index+1].NodeKey)
		}
	}
	node, _ := perspective.Node(request.NodeKey)
	request.Labels.Perspective = perspective.Label
	request.Labels.Node = node.Label
	return request, nil
}

func ExportRequestFromState(spec Spec, state State, mode ExportMode) (ExportRequest, error) {
	selection, err := ActiveSelection(spec, state)
	if err != nil {
		return ExportRequest{}, err
	}
	request := ExportRequest{
		Mode:       mode,
		ExplorerID: selection.ExplorerID,
		BranchKey:  selection.BranchKey,
	}
	if mode == ExportCurrentView {
		request.PerspectiveKey = selection.PerspectiveKey
		request.Path = selection.Path
		request.Steps = selection.Steps
		request.NodeKey = selection.NodeKey
	}
	return ResolveExportRequest(spec, request)
}

func nodeHasTarget(node Node, target PathStep) bool {
	for _, edge := range node.Edges {
		if edge.ToNode == target.NodeKey && (target.PointKey == "" || edge.PointKey == target.PointKey) {
			return true
		}
	}
	if !node.DynamicEdges || strings.TrimSpace(target.PointKey) == "" {
		return false
	}
	for _, candidate := range node.DynamicTargets {
		if candidate == target.NodeKey {
			return true
		}
	}
	return false
}

func exportPathSteps(request ExportRequest) []PathStep {
	if len(request.Steps) > 0 {
		return request.Steps
	}
	steps := make([]PathStep, 0, len(request.Path))
	for _, nodeKey := range request.Path {
		steps = append(steps, PathStep{NodeKey: nodeKey})
	}
	return steps
}
