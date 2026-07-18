package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ExplorationLoader is implemented by the host application. It resolves one
// explorer node without coupling Lens to application metrics or repositories.
type ExplorationLoader interface {
	LoadExploration(context.Context, ExplorationLoadRequest) (ExplorationDefinition, error)
}

type ExplorationLoadRequest struct {
	ExplorerID     string
	BranchKey      string
	PerspectiveKey string
	Path           []string
	Steps          []explore.PathStep
	Variables      map[string]any
}

type ExplorationDefinition struct {
	Dashboard     lens.DashboardSpec
	PanelID       string
	ResolvedEdges []explore.Edge
}

type ExplorationResult struct {
	Explorer explore.Spec
	Branch   explore.Branch
	View     explore.Perspective
	Path     []string
	Steps    []explore.PathStep
	Panel    *PanelResult
	Edges    []explore.Edge
}

// ExplorationFragmentRequest is transport-neutral input for an HTTP, RPC, or
// test adapter. Dashboard is the prepared root definition; Runtime carries the
// same authorization/cache identity used by ordinary panel fragments.
type ExplorationFragmentRequest struct {
	Dashboard lens.DashboardSpec
	Load      ExplorationLoadRequest
	Runtime   Request
}

type ExplorationFragmentResponse struct {
	Result *ExplorationResult
	Panel  *PanelResult
}

type ExplorationFragmentHandler struct {
	Runtime *Runtime
	Loader  ExplorationLoader
}

func (h ExplorationFragmentHandler) Handle(ctx context.Context, req ExplorationFragmentRequest) (*ExplorationFragmentResponse, error) {
	if h.Runtime == nil {
		return nil, fmt.Errorf("exploration fragment runtime is required")
	}
	result, err := h.Runtime.ExecuteExploration(ctx, req.Dashboard, h.Loader, req.Load, req.Runtime)
	if err != nil {
		return nil, err
	}
	return &ExplorationFragmentResponse{Result: result, Panel: result.Panel}, nil
}

// ExecuteExploration loads and executes exactly one explorer node. The caller
// supplies the ordinary runtime request so tenant scope, variables, cache
// identity, locale, and timezone follow the same contract as dashboard panels.
func (r *Runtime) ExecuteExploration(
	ctx context.Context,
	dashboard lens.DashboardSpec,
	loader ExplorationLoader,
	loadReq ExplorationLoadRequest,
	runtimeReq Request,
) (*ExplorationResult, error) {
	const op serrors.Op = "lens/runtime.ExecuteExploration"

	explorerSpec, branch, perspective, err := resolveExploration(dashboard, loadReq)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if loader == nil {
		return nil, serrors.E(op, fmt.Errorf("exploration loader is required"))
	}
	loadReq.PerspectiveKey = perspective.Key
	loadReq.Steps = normalizeExplorationSteps(loadReq.Path, loadReq.Steps, perspective.RootNode)
	loadReq.Path = explorationNodePath(loadReq.Steps)
	loadReq.Variables = cloneMap(loadReq.Variables)
	definition, err := loader.LoadExploration(ctx, loadReq)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if strings.TrimSpace(definition.PanelID) == "" {
		return nil, serrors.E(op, fmt.Errorf("exploration loader returned an empty panel id"))
	}
	executed, err := r.Execute(ctx, definition.Dashboard, runtimeReq, PanelScope(definition.PanelID))
	if err != nil {
		return nil, serrors.E(op, err)
	}
	panelResult := executed.Panel(definition.PanelID)
	if panelResult == nil {
		return nil, serrors.E(op, fmt.Errorf("exploration loader panel %q was not executed", definition.PanelID))
	}
	nodeKey := loadReq.Steps[len(loadReq.Steps)-1].NodeKey
	node, _ := perspective.Node(nodeKey)
	edges, err := resolvedExplorationEdges(explorerSpec.ID, perspective, node, definition.ResolvedEdges, panelResult)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return &ExplorationResult{
		Explorer: explorerSpec,
		Branch:   branch,
		View:     perspective,
		Path:     append([]string(nil), loadReq.Path...),
		Steps:    append([]explore.PathStep(nil), loadReq.Steps...),
		Panel:    panelResult,
		Edges:    edges,
	}, nil
}

func resolveExploration(dashboard lens.DashboardSpec, req ExplorationLoadRequest) (explore.Spec, explore.Branch, explore.Perspective, error) {
	var explorerSpec explore.Spec
	found := false
	for _, candidate := range dashboard.Explorers {
		if candidate.ID == req.ExplorerID {
			explorerSpec = candidate
			found = true
			break
		}
	}
	if !found {
		return explore.Spec{}, explore.Branch{}, explore.Perspective{}, fmt.Errorf("explorer %q not found", req.ExplorerID)
	}
	branch, ok := explorerSpec.Branch(req.BranchKey)
	if !ok {
		return explore.Spec{}, explore.Branch{}, explore.Perspective{}, fmt.Errorf("explorer %s has no branch %q", explorerSpec.ID, req.BranchKey)
	}
	perspectiveKey := req.PerspectiveKey
	if perspectiveKey == "" {
		perspectiveKey = branch.DefaultPerspective
	}
	perspective, ok := branch.Perspective(perspectiveKey)
	if !ok {
		return explore.Spec{}, explore.Branch{}, explore.Perspective{}, fmt.Errorf("explorer %s branch %s has no perspective %q", explorerSpec.ID, branch.Key, perspectiveKey)
	}
	steps := normalizeExplorationSteps(req.Path, req.Steps, perspective.RootNode)
	if len(steps) > 0 {
		if steps[0].NodeKey != perspective.RootNode {
			return explore.Spec{}, explore.Branch{}, explore.Perspective{}, fmt.Errorf("explorer %s perspective %s path must start at root node %q", explorerSpec.ID, perspective.Key, perspective.RootNode)
		}
		for index, step := range steps {
			nodeKey := step.NodeKey
			node, ok := perspective.Node(nodeKey)
			if !ok {
				return explore.Spec{}, explore.Branch{}, explore.Perspective{}, fmt.Errorf("explorer %s perspective %s has no node %q", explorerSpec.ID, perspective.Key, nodeKey)
			}
			if index+1 < len(steps) && !nodeTargets(node, steps[index+1]) {
				return explore.Spec{}, explore.Branch{}, explore.Perspective{}, fmt.Errorf("explorer %s perspective %s path cannot move from %q to %q", explorerSpec.ID, perspective.Key, nodeKey, steps[index+1].NodeKey)
			}
		}
	}
	return explorerSpec, branch, perspective, nil
}

func nodeTargets(node explore.Node, target explore.PathStep) bool {
	for _, edge := range node.Edges {
		if edge.ToNode == target.NodeKey && (target.PointKey == "" || edge.PointKey == target.PointKey) {
			return true
		}
	}
	if !node.DynamicEdges || strings.TrimSpace(target.PointKey) == "" {
		return false
	}
	return containsExplorationTarget(node.DynamicTargets, target.NodeKey)
}

func normalizeExplorationSteps(path []string, steps []explore.PathStep, root string) []explore.PathStep {
	if len(steps) > 0 {
		return append([]explore.PathStep(nil), steps...)
	}
	for _, key := range path {
		steps = append(steps, explore.PathStep{NodeKey: key})
	}
	if len(steps) == 0 {
		steps = []explore.PathStep{{NodeKey: root}}
	}
	return steps
}

func explorationNodePath(steps []explore.PathStep) []string {
	path := make([]string, 0, len(steps))
	for _, step := range steps {
		path = append(path, step.NodeKey)
	}
	return path
}

func resolvedExplorationEdges(explorerID string, perspective explore.Perspective, node explore.Node, resolved []explore.Edge, panelResult *PanelResult) ([]explore.Edge, error) {
	if len(resolved) > 0 && !node.DynamicEdges {
		return nil, fmt.Errorf("exploration node %q does not allow dynamic edges", node.Key)
	}
	edges := append(append([]explore.Edge(nil), node.Edges...), resolved...)
	if len(edges) == 0 {
		return nil, nil
	}
	if panelResult.Panel.Fields.ID.Empty() {
		return nil, fmt.Errorf("exploration node %q with edges requires loaded panel %q to define an id field", node.Key, panelResult.Panel.ID)
	}
	if panelResult.Error != nil {
		return edges, nil //nolint:nilerr // Panel errors are rendered by the fragment; edge validation requires successful rows.
	}
	primary := panelResult.Frames.Primary()
	if primary == nil {
		return nil, fmt.Errorf("exploration node %q with edges requires loaded panel %q rows", node.Key, panelResult.Panel.ID)
	}
	idName := panelResult.Panel.Fields.ID.Name()
	field, ok := primary.Field(idName)
	if !ok {
		return nil, fmt.Errorf("exploration node %q loaded panel %q is missing id field %q", node.Key, panelResult.Panel.ID, idName)
	}
	points := make(map[string]struct{}, len(field.Values))
	for index, value := range field.Values {
		key, ok := value.(string)
		if !ok || strings.TrimSpace(key) == "" || key != strings.TrimSpace(key) {
			return nil, fmt.Errorf("exploration node %q loaded panel %q id field %q row %d requires a nonblank string", node.Key, panelResult.Panel.ID, idName, index)
		}
		if _, duplicate := points[key]; duplicate {
			return nil, fmt.Errorf("exploration node %q loaded panel %q id field %q has duplicate point %q", node.Key, panelResult.Panel.ID, idName, key)
		}
		points[key] = struct{}{}
	}
	seen := make(map[string]struct{}, len(edges))
	for index, edge := range edges {
		owner := "explorer " + explorerID + " node " + node.Key + " edge " + edge.PointKey
		if strings.TrimSpace(edge.PointKey) == "" || edge.PointKey != strings.TrimSpace(edge.PointKey) {
			return nil, fmt.Errorf("%s requires a nonblank point key", owner)
		}
		if _, duplicate := seen[edge.PointKey]; duplicate {
			return nil, fmt.Errorf("explorer %s node %s has duplicate resolved edge point %q", explorerID, node.Key, edge.PointKey)
		}
		seen[edge.PointKey] = struct{}{}
		if _, ok := points[edge.PointKey]; !ok {
			return nil, fmt.Errorf("%s is missing from loaded panel %q id field %q", owner, panelResult.Panel.ID, idName)
		}
		if (strings.TrimSpace(edge.ToNode) == "") == (edge.Action == nil) {
			return nil, fmt.Errorf("%s requires exactly one of target node or action", owner)
		}
		if edge.ToNode != "" {
			if _, ok := perspective.Node(edge.ToNode); !ok {
				return nil, fmt.Errorf("%s references missing node %q", owner, edge.ToNode)
			}
			if index >= len(node.Edges) && !containsExplorationTarget(node.DynamicTargets, edge.ToNode) {
				return nil, fmt.Errorf("%s references undeclared dynamic target %q", owner, edge.ToNode)
			}
		}
		if err := validateAction(owner, edge.Action, actionValidationOptions{allowFieldSources: true}); err != nil {
			return nil, err
		}
	}
	return edges, nil
}

func containsExplorationTarget(targets []string, target string) bool {
	for _, candidate := range targets {
		if candidate == target {
			return true
		}
	}
	return false
}
