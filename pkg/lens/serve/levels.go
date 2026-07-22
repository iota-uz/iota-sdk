package serve

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type levelTarget struct {
	explorerID     string
	branchKey      string
	perspectiveKey string
	nodeKey        string
	path           []string
	panel          panel.Spec
	ref            document.FrameRef
	evidence       bool
	perspective    explore.Perspective
	levelKey       document.NodeKey
	// points are the concrete selections the request path carries between its
	// node steps (e.g. the "2026" in [root, "2026", detail]). A level reached
	// through a point aggregates only that selection, so its frame must never
	// be cached, or served, under the node's unparameterised reference.
	points []string
}

// cacheRef keys the snapshot frame cache. A point-free request keeps the plain
// node reference — it is the same frame the document inlines — while a
// point-parameterised request gets its own key so sibling selections do not
// replay each other's frames.
func (t levelTarget) cacheRef() document.FrameRef {
	if len(t.points) == 0 {
		return t.ref
	}
	return document.FrameRef(string(t.ref) + "@" + strings.Join(t.points, "/"))
}

// selectionPoints extracts the point selections from a query path. Two wire
// shapes arrive here: bare alternating entries ([root, "2026", detail]) and
// the React runtime's qualified entries, where every structural step is a
// "/"-prefix of a later entry. Whatever is neither structural nor a node of
// the resolved perspective is a selection, in path order.
func selectionPoints(path []string, perspective explore.Perspective) []string {
	points := make([]string, 0)
	for index, entry := range path {
		part := lastPathSegment(entry)
		if part == "" {
			continue
		}
		structural := false
		for next := index + 1; next < len(path); next++ {
			if strings.HasPrefix(path[next], entry+"/") {
				structural = true
				break
			}
		}
		if structural {
			continue
		}
		if _, ok := perspective.Node(part); ok {
			continue
		}
		points = append(points, part)
	}
	return points
}

func lastPathSegment(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if index := strings.LastIndexByte(value, '/'); index >= 0 {
		return value[index+1:]
	}
	return value
}

func inlineTargets(spec lens.DashboardSpec, inlineDepth int) []levelTarget {
	targets := make([]levelTarget, 0)
	for _, explorerSpec := range spec.Explorers {
		for _, branch := range explorerSpec.Branches {
			for _, perspective := range branch.Perspectives {
				depths := perspectiveDepths(perspective)
				for _, node := range perspective.Nodes {
					if node.Panel == nil || depths[node.Key] > inlineDepth || isEvidence(perspective, node) {
						continue
					}
					targets = append(targets, makeTarget(explorerSpec.ID, branch.Key, perspective, node))
				}
			}
		}
	}
	return targets
}

func resolveTarget(spec lens.DashboardSpec, path document.NodePath, requestedPerspective string) (levelTarget, error) {
	last := strings.TrimSpace(string(path[len(path)-1]))
	requestedPerspective = strings.TrimSpace(requestedPerspective)
	candidates := make([]levelTarget, 0, 1)
	for _, explorerSpec := range spec.Explorers {
		for _, branch := range explorerSpec.Branches {
			for _, perspective := range branch.Perspectives {
				perspectiveID := qualified(explorerSpec.ID, branch.Key, perspective.Key)
				if requestedPerspective != "" && requestedPerspective != perspective.Key && requestedPerspective != perspectiveID {
					continue
				}
				for _, node := range perspective.Nodes {
					if node.Panel == nil {
						continue
					}
					nodeID := qualified(perspectiveID, node.Key)
					if last == node.Key || last == nodeID {
						candidates = append(candidates, makeTarget(explorerSpec.ID, branch.Key, perspective, node))
					}
				}
				for _, source := range perspective.Nodes {
					for _, edge := range source.Edges {
						pointID := qualified(perspectiveID, source.Key, edge.PointKey)
						if edge.ToNode == "" || last != edge.PointKey && last != pointID {
							continue
						}
						if node, ok := perspective.Node(edge.ToNode); ok && node.Panel != nil {
							candidates = append(candidates, makeTarget(explorerSpec.ID, branch.Key, perspective, node))
						}
					}
				}
			}
		}
	}
	if len(candidates) == 0 {
		return levelTarget{}, fmt.Errorf("requested path does not identify an executable level")
	}
	unique := candidates[:0]
	seen := make(map[document.FrameRef]struct{}, len(candidates))
	for _, candidate := range candidates {
		if _, ok := seen[candidate.ref]; ok {
			continue
		}
		seen[candidate.ref] = struct{}{}
		candidate.path = nodePath(path)
		unique = append(unique, candidate)
	}
	if len(unique) != 1 {
		return levelTarget{}, fmt.Errorf("requested path is ambiguous; perspective is required")
	}
	unique[0].points = selectionPoints(unique[0].path, unique[0].perspective)
	return unique[0], nil
}

func makeTarget(explorerID, branchKey string, perspective explore.Perspective, node explore.Node) levelTarget {
	perspectiveID := qualified(explorerID, branchKey, perspective.Key)
	return levelTarget{
		explorerID: explorerID, branchKey: branchKey, perspectiveKey: perspective.Key, nodeKey: node.Key,
		path: []string{node.Key}, panel: *node.Panel,
		ref: document.FrameRef("explore:" + perspectiveID + ":" + node.Key), evidence: isEvidence(perspective, node),
		perspective: perspective,
		levelKey:    document.NodeKey(qualified(perspectiveID, node.Key)),
	}
}

func isEvidence(perspective explore.Perspective, node explore.Node) bool {
	leaf := !node.DynamicEdges
	for _, edge := range node.Edges {
		if edge.ToNode != "" {
			leaf = false
			break
		}
	}
	return leaf && (perspective.Semantics == explore.SemanticsEvidence || node.Panel != nil && node.Panel.Kind == panel.KindTable)
}

func perspectiveDepths(perspective explore.Perspective) map[string]int {
	const unseen = int(^uint(0) >> 1)
	depths := make(map[string]int, len(perspective.Nodes))
	for _, node := range perspective.Nodes {
		depths[node.Key] = unseen
	}
	depths[perspective.RootNode] = 0
	changed := true
	for changed {
		changed = false
		for _, node := range perspective.Nodes {
			depth := depths[node.Key]
			if depth == unseen {
				continue
			}
			for _, edge := range node.Edges {
				if edge.ToNode != "" && depth+1 < depths[edge.ToNode] {
					depths[edge.ToNode] = depth + 1
					changed = true
				}
			}
			for _, target := range node.DynamicTargets {
				if depth+1 < depths[target] {
					depths[target] = depth + 1
					changed = true
				}
			}
		}
	}
	return depths
}

func (h *Handlers) executeLevel(ctx context.Context, base lensruntime.Request, params map[string]any, target levelTarget, page int) (*lensruntime.PanelResult, error) {
	const op serrors.Op = "lens/serve.executeLevel"
	req := scopedRuntimeRequest(base, params, target, page, h.pageSize)
	result, err := h.engine.Execute(ctx, levelSpec(h.spec, target.panel), req, lensruntime.PanelScope(target.panel.ID))
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if result == nil || result.Panel(target.panel.ID) == nil {
		return nil, serrors.E(op, fmt.Errorf("panel %q was not executed", target.panel.ID))
	}
	panelResult := result.Panel(target.panel.ID)
	if panelResult.Error != nil {
		return panelResult, serrors.E(op, panelResult.Error)
	}
	return panelResult, nil
}

func levelSpec(spec lens.DashboardSpec, target panel.Spec) lens.DashboardSpec {
	spec.Rows = []lens.RowSpec{{Panels: []panel.Spec{target}}}
	spec.Explorers = nil
	return spec
}

func scopedRuntimeRequest(base lensruntime.Request, params map[string]any, target levelTarget, page, pageSize int) lensruntime.Request {
	base.Overrides = variableParams(params)
	base.Request = cloneValues(base.Request)
	base.Request.Set(lensexplorerParam, target.explorerID)
	base.Request.Set(lensbranchParam, target.branchKey)
	base.Request.Set(lensperspectiveParam, target.perspectiveKey)
	base.Request.Set(lensnodeParam, target.nodeKey)
	base.Request.Del(lenspathParam)
	for _, item := range target.path {
		base.Request.Add(lenspathParam, item)
	}
	if page > 0 {
		base.Request.Set(lensruntime.TablePaginationPanelQuery, target.panel.ID)
		base.Request.Set(lensruntime.TablePaginationPageQuery, strconv.Itoa(page))
		base.Request.Set(lensruntime.TablePaginationLimitQuery, strconv.Itoa(pageSize))
	}
	return base
}

const (
	lensexplorerParam    = "lens_explorer"
	lensbranchParam      = "lens_explore_branch"
	lensperspectiveParam = "lens_explore_perspective"
	lenspathParam        = "lens_explore_path"
	lensnodeParam        = "lens_explore_node"
)

func qualified(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), "/")
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, "/")
}

func nodePath(path document.NodePath) []string {
	result := make([]string, 0, len(path))
	for _, item := range path {
		value := strings.TrimSpace(string(item))
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func cloneValues(values url.Values) url.Values {
	result := make(url.Values, len(values))
	for key, items := range values {
		result[key] = append([]string(nil), items...)
	}
	return result
}
