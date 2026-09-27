package lens

import (
	"fmt"
	"slices"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
)

// ExecutionSurfaceKind identifies an independently activatable view compiled
// from one DashboardSpec. Transport adapters may open several surfaces, but
// every one belongs to the same snapshot execution graph.
type ExecutionSurfaceKind string

const (
	ExecutionSurfaceRoot        ExecutionSurfaceKind = "root"
	ExecutionSurfaceExploration ExecutionSurfaceKind = "exploration"
)

// ExecutionSurface is a pure scheduling description. It intentionally has no
// operator-supplied priority: visual order and graph shape are the only inputs.
type ExecutionSurface struct {
	Key             string
	Kind            ExecutionSurfaceKind
	PanelID         string
	DatasetClosure  []string
	VisualOrder     int
	Depth           int
	ParentKey       string
	DynamicChildren bool
}

// ExecutionPlan is the immutable surface graph compiled from DashboardSpec.
type ExecutionPlan struct {
	Surfaces []ExecutionSurface
	byKey    map[string]int
}

func (p ExecutionPlan) Surface(key string) (ExecutionSurface, bool) {
	index, ok := p.byKey[key]
	if !ok || index < 0 || index >= len(p.Surfaces) {
		return ExecutionSurface{}, false
	}
	return p.Surfaces[index], true
}

// CompileExecutionPlan derives root and exploration surfaces, including each
// panel's transitive dataset dependency closure, without executing data.
func CompileExecutionPlan(spec DashboardSpec) (ExecutionPlan, error) {
	datasets := make(map[string]DatasetSpec, len(spec.Datasets))
	for _, dataset := range spec.Datasets {
		name := strings.TrimSpace(dataset.Name)
		if name == "" {
			return ExecutionPlan{}, fmt.Errorf("execution plan contains an unnamed dataset")
		}
		datasets[name] = dataset
	}
	closure := func(name string) ([]string, error) {
		seen := map[string]bool{}
		active := map[string]bool{}
		ordered := make([]string, 0)
		var visit func(string) error
		visit = func(current string) error {
			current = strings.TrimSpace(current)
			if current == "" || seen[current] {
				return nil
			}
			if active[current] {
				return fmt.Errorf("execution plan dataset dependency cycle at %q", current)
			}
			dataset, ok := datasets[current]
			if !ok {
				return fmt.Errorf("execution plan references missing dataset %q", current)
			}
			active[current] = true
			for _, dependency := range dataset.DependsOn {
				if err := visit(dependency); err != nil {
					return err
				}
			}
			active[current] = false
			seen[current] = true
			ordered = append(ordered, current)
			return nil
		}
		if err := visit(name); err != nil {
			return nil, err
		}
		return ordered, nil
	}

	plan := ExecutionPlan{byKey: make(map[string]int)}
	add := func(surface ExecutionSurface) error {
		if _, duplicate := plan.byKey[surface.Key]; duplicate {
			return fmt.Errorf("execution plan has duplicate surface %q", surface.Key)
		}
		plan.byKey[surface.Key] = len(plan.Surfaces)
		plan.Surfaces = append(plan.Surfaces, surface)
		return nil
	}
	visualOrder := 0
	var addRoot func(panel.Spec, int) error
	addRoot = func(candidate panel.Spec, row int) error {
		if !candidate.Kind.IsContainer() {
			dependencies, err := closure(candidate.Dataset)
			if err != nil && candidate.Dataset != "" {
				return fmt.Errorf("panel %s: %w", candidate.ID, err)
			}
			if err := add(ExecutionSurface{
				Key: "root:" + candidate.ID, Kind: ExecutionSurfaceRoot, PanelID: candidate.ID,
				DatasetClosure: dependencies, VisualOrder: visualOrder, Depth: row,
			}); err != nil {
				return err
			}
			visualOrder++
		}
		for _, child := range candidate.Children {
			if err := addRoot(child, row); err != nil {
				return err
			}
		}
		return nil
	}
	for row, layout := range spec.Rows {
		for _, candidate := range layout.Panels {
			if err := addRoot(candidate, row); err != nil {
				return ExecutionPlan{}, err
			}
		}
	}

	for _, explorer := range spec.Explorers {
		for _, branch := range explorer.Branches {
			for _, perspective := range branch.Perspectives {
				depths := executionPerspectiveDepths(perspective.RootNode, perspective.Nodes)
				for order, node := range perspective.Nodes {
					if node.Panel == nil {
						continue
					}
					dependencies, err := closure(node.Panel.Dataset)
					if err != nil && node.Panel.Dataset != "" {
						return ExecutionPlan{}, fmt.Errorf("exploration panel %s: %w", node.Panel.ID, err)
					}
					key := strings.Join([]string{"explore", explorer.ID, branch.Key, perspective.Key, node.Key}, ":")
					parent := ""
					for _, source := range perspective.Nodes {
						for _, edge := range source.Edges {
							if edge.ToNode == node.Key {
								parent = strings.Join([]string{"explore", explorer.ID, branch.Key, perspective.Key, source.Key}, ":")
							}
						}
					}
					if err := add(ExecutionSurface{
						Key: key, Kind: ExecutionSurfaceExploration, PanelID: node.Panel.ID,
						DatasetClosure: dependencies, VisualOrder: order, Depth: depths[node.Key], ParentKey: parent,
						DynamicChildren: node.DynamicEdges || len(node.DynamicTargets) > 0,
					}); err != nil {
						return ExecutionPlan{}, err
					}
				}
			}
		}
	}
	return plan, nil
}

func executionPerspectiveDepths(root string, nodes []explore.Node) map[string]int {
	const unseen = int(^uint(0) >> 1)
	depths := make(map[string]int, len(nodes))
	for _, node := range nodes {
		depths[node.Key] = unseen
	}
	depths[root] = 0
	changed := true
	for changed {
		changed = false
		for _, node := range nodes {
			depth := depths[node.Key]
			if depth == unseen {
				continue
			}
			targets := append([]string(nil), node.DynamicTargets...)
			for _, edge := range node.Edges {
				targets = append(targets, edge.ToNode)
			}
			for _, target := range targets {
				if target != "" && depth+1 < depths[target] {
					depths[target] = depth + 1
					changed = true
				}
			}
		}
	}
	for key, depth := range depths {
		if depth == unseen {
			depths[key] = len(nodes)
		}
	}
	return depths
}

// SharedDatasets returns the dependency nodes reused by two surfaces.
func (p ExecutionPlan) SharedDatasets(left, right string) []string {
	a, aOK := p.Surface(left)
	b, bOK := p.Surface(right)
	if !aOK || !bOK {
		return nil
	}
	shared := make([]string, 0)
	for _, dataset := range a.DatasetClosure {
		if slices.Contains(b.DatasetClosure, dataset) {
			shared = append(shared, dataset)
		}
	}
	return shared
}
