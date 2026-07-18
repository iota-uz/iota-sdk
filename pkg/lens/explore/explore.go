// Package explore defines renderer-independent, multi-perspective metric exploration.
package explore

import (
	"fmt"
	"math"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/exportmeta"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
)

type Semantics string

const (
	SemanticsPartition      Semantics = "partition"
	SemanticsReconciliation Semantics = "reconciliation"
	SemanticsSeries         Semantics = "series"
	SemanticsEvidence       Semantics = "evidence"
)

type Spec struct {
	ID           string   `json:"id"`
	HostPanelID  string   `json:"hostPanelId"`
	ExpandedSpan int      `json:"expandedSpan,omitempty"`
	Branches     []Branch `json:"branches"`
}

type Branch struct {
	Key                string        `json:"key"`
	Label              string        `json:"label"`
	DefaultPerspective string        `json:"defaultPerspective"`
	Perspectives       []Perspective `json:"perspectives"`
}

type Perspective struct {
	Key       string          `json:"key"`
	Label     string          `json:"label"`
	Semantics Semantics       `json:"semantics"`
	RootNode  string          `json:"rootNode"`
	Nodes     []Node          `json:"nodes"`
	Export    exportmeta.Spec `json:"export,omitempty"`
}

type Node struct {
	Key            string        `json:"key"`
	Label          string        `json:"label"`
	Panel          *panel.Spec   `json:"panel,omitempty"`
	Load           *LoadSpec     `json:"load,omitempty"`
	Edges          []Edge        `json:"edges,omitempty"`
	DynamicEdges   bool          `json:"dynamicEdges,omitempty"`
	DynamicTargets []string      `json:"dynamicTargets,omitempty"`
	Check          *BalanceCheck `json:"check,omitempty"`
}

type PathStep struct {
	NodeKey  string `json:"nodeKey"`
	PointKey string `json:"pointKey,omitempty"`
}

type LoadSpec struct {
	URL           string `json:"url"`
	Method        string `json:"method,omitempty"`
	PreserveQuery bool   `json:"preserveQuery,omitempty"`
}

type Edge struct {
	PointKey string       `json:"pointKey"`
	ToNode   string       `json:"toNode,omitempty"`
	Action   *action.Spec `json:"action,omitempty"`
}

type BalanceCheck struct {
	Expected  float64 `json:"expected"`
	Actual    float64 `json:"actual"`
	Tolerance float64 `json:"tolerance,omitempty"`
}

func (s Spec) Validate() error {
	if err := validKey("explorer id", s.ID); err != nil {
		return err
	}
	if err := validKey("explorer host panel id", s.HostPanelID); err != nil {
		return err
	}
	if s.ExpandedSpan < 0 || s.ExpandedSpan > 12 {
		return fmt.Errorf("explorer %s expanded span must be between 1 and 12 when configured", s.ID)
	}
	if len(s.Branches) == 0 {
		return fmt.Errorf("explorer %s requires at least one branch", s.ID)
	}
	seen := make(map[string]struct{}, len(s.Branches))
	for i := range s.Branches {
		branch := &s.Branches[i]
		if err := validKey("branch key", branch.Key); err != nil {
			return fmt.Errorf("explorer %s: %w", s.ID, err)
		}
		if _, ok := seen[branch.Key]; ok {
			return fmt.Errorf("explorer %s has duplicate branch %q", s.ID, branch.Key)
		}
		seen[branch.Key] = struct{}{}
		if err := branch.validate(s.ID); err != nil {
			return err
		}
	}
	return nil
}

func (b Branch) validate(explorerID string) error {
	if strings.TrimSpace(b.Label) == "" {
		return fmt.Errorf("explorer %s branch %s requires a label", explorerID, b.Key)
	}
	if len(b.Perspectives) == 0 {
		return fmt.Errorf("explorer %s branch %s requires at least one perspective", explorerID, b.Key)
	}
	seen := make(map[string]struct{}, len(b.Perspectives))
	for i := range b.Perspectives {
		perspective := &b.Perspectives[i]
		if err := validKey("perspective key", perspective.Key); err != nil {
			return fmt.Errorf("explorer %s branch %s: %w", explorerID, b.Key, err)
		}
		if _, ok := seen[perspective.Key]; ok {
			return fmt.Errorf("explorer %s branch %s has duplicate perspective %q", explorerID, b.Key, perspective.Key)
		}
		seen[perspective.Key] = struct{}{}
		if err := perspective.validate(explorerID, b.Key); err != nil {
			return err
		}
	}
	if _, ok := seen[b.DefaultPerspective]; !ok {
		return fmt.Errorf("explorer %s branch %s references missing default perspective %q", explorerID, b.Key, b.DefaultPerspective)
	}
	return nil
}

func (p Perspective) validate(explorerID, branchKey string) error {
	if strings.TrimSpace(p.Label) == "" {
		return fmt.Errorf("explorer %s branch %s perspective %s requires a label", explorerID, branchKey, p.Key)
	}
	switch p.Semantics {
	case SemanticsPartition, SemanticsReconciliation, SemanticsSeries, SemanticsEvidence:
	default:
		return fmt.Errorf("explorer %s branch %s perspective %s has unsupported semantics %q", explorerID, branchKey, p.Key, p.Semantics)
	}
	if len(p.Nodes) == 0 {
		return fmt.Errorf("explorer %s branch %s perspective %s requires at least one node", explorerID, branchKey, p.Key)
	}
	nodes := make(map[string]Node, len(p.Nodes))
	for _, node := range p.Nodes {
		if err := validKey("node key", node.Key); err != nil {
			return fmt.Errorf("explorer %s branch %s perspective %s: %w", explorerID, branchKey, p.Key, err)
		}
		if _, ok := nodes[node.Key]; ok {
			return fmt.Errorf("explorer %s branch %s perspective %s has duplicate node %q", explorerID, branchKey, p.Key, node.Key)
		}
		nodes[node.Key] = node
	}
	if _, ok := nodes[p.RootNode]; !ok {
		return fmt.Errorf("explorer %s branch %s perspective %s references missing root node %q", explorerID, branchKey, p.Key, p.RootNode)
	}
	for _, node := range p.Nodes {
		if err := node.validate(explorerID, branchKey, p.Key, nodes); err != nil {
			return err
		}
	}
	return validateAcyclic(explorerID, branchKey, p, nodes)
}

func (n Node) validate(explorerID, branchKey, perspectiveKey string, nodes map[string]Node) error {
	if strings.TrimSpace(n.Label) == "" {
		return fmt.Errorf("explorer %s branch %s perspective %s node %s requires a label", explorerID, branchKey, perspectiveKey, n.Key)
	}
	if (n.Panel == nil) == (n.Load == nil) {
		return fmt.Errorf("explorer %s branch %s perspective %s node %s requires exactly one of panel or load", explorerID, branchKey, perspectiveKey, n.Key)
	}
	if n.Panel != nil && (len(n.Edges) > 0 || n.DynamicEdges) && n.Panel.Fields.ID.Empty() {
		return fmt.Errorf("explorer %s branch %s perspective %s node %s with edges requires panel id field", explorerID, branchKey, perspectiveKey, n.Key)
	}
	if n.Load != nil {
		if strings.TrimSpace(n.Load.URL) == "" {
			return fmt.Errorf("explorer %s branch %s perspective %s node %s load requires url", explorerID, branchKey, perspectiveKey, n.Key)
		}
		switch strings.ToUpper(strings.TrimSpace(n.Load.Method)) {
		case "", "GET", "POST":
		default:
			return fmt.Errorf("explorer %s branch %s perspective %s node %s load has unsupported method %q", explorerID, branchKey, perspectiveKey, n.Key, n.Load.Method)
		}
	}
	if n.Check != nil {
		if !finite(n.Check.Expected) || !finite(n.Check.Actual) || !finite(n.Check.Tolerance) || n.Check.Tolerance < 0 {
			return fmt.Errorf("explorer %s branch %s perspective %s node %s has invalid balance check", explorerID, branchKey, perspectiveKey, n.Key)
		}
		if math.Abs(n.Check.Expected-n.Check.Actual) > n.Check.Tolerance {
			return fmt.Errorf("explorer %s branch %s perspective %s node %s is out of balance", explorerID, branchKey, perspectiveKey, n.Key)
		}
	}
	seen := make(map[string]struct{}, len(n.Edges))
	for _, edge := range n.Edges {
		if err := validKey("edge point key", edge.PointKey); err != nil {
			return fmt.Errorf("explorer %s branch %s perspective %s node %s: %w", explorerID, branchKey, perspectiveKey, n.Key, err)
		}
		if _, ok := seen[edge.PointKey]; ok {
			return fmt.Errorf("explorer %s branch %s perspective %s node %s has duplicate edge point %q", explorerID, branchKey, perspectiveKey, n.Key, edge.PointKey)
		}
		seen[edge.PointKey] = struct{}{}
		if (strings.TrimSpace(edge.ToNode) == "") == (edge.Action == nil) {
			return fmt.Errorf("explorer %s branch %s perspective %s node %s edge %s requires exactly one of target node or action", explorerID, branchKey, perspectiveKey, n.Key, edge.PointKey)
		}
		if edge.ToNode != "" {
			if _, ok := nodes[edge.ToNode]; !ok {
				return fmt.Errorf("explorer %s branch %s perspective %s node %s edge %s references missing node %q", explorerID, branchKey, perspectiveKey, n.Key, edge.PointKey, edge.ToNode)
			}
		}
	}
	dynamicTargets := make(map[string]struct{}, len(n.DynamicTargets))
	for _, target := range n.DynamicTargets {
		if err := validKey("dynamic target node", target); err != nil {
			return fmt.Errorf("explorer %s branch %s perspective %s node %s: %w", explorerID, branchKey, perspectiveKey, n.Key, err)
		}
		if _, duplicate := dynamicTargets[target]; duplicate {
			return fmt.Errorf("explorer %s branch %s perspective %s node %s has duplicate dynamic target %q", explorerID, branchKey, perspectiveKey, n.Key, target)
		}
		dynamicTargets[target] = struct{}{}
		if _, ok := nodes[target]; !ok {
			return fmt.Errorf("explorer %s branch %s perspective %s node %s references missing dynamic target %q", explorerID, branchKey, perspectiveKey, n.Key, target)
		}
	}
	if len(n.DynamicTargets) > 0 && !n.DynamicEdges {
		return fmt.Errorf("explorer %s branch %s perspective %s node %s has dynamic targets without dynamic edges", explorerID, branchKey, perspectiveKey, n.Key)
	}
	return nil
}

func validateAcyclic(explorerID, branchKey string, perspective Perspective, nodes map[string]Node) error {
	visiting := make(map[string]bool, len(nodes))
	visited := make(map[string]bool, len(nodes))
	var visit func(string) error
	visit = func(key string) error {
		if visited[key] {
			return nil
		}
		if visiting[key] {
			return fmt.Errorf("explorer %s branch %s perspective %s contains a node cycle at %q", explorerID, branchKey, perspective.Key, key)
		}
		visiting[key] = true
		for _, edge := range nodes[key].Edges {
			if edge.ToNode != "" {
				if err := visit(edge.ToNode); err != nil {
					return err
				}
			}
		}
		for _, target := range nodes[key].DynamicTargets {
			if err := visit(target); err != nil {
				return err
			}
		}
		visiting[key] = false
		visited[key] = true
		return nil
	}
	return visit(perspective.RootNode)
}

func validKey(name, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s is required", name)
	}
	if trimmed != value {
		return fmt.Errorf("%s %q has surrounding whitespace", name, value)
	}
	return nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func (s Spec) Branch(key string) (Branch, bool) {
	for _, branch := range s.Branches {
		if branch.Key == key {
			return branch, true
		}
	}
	return Branch{}, false
}

func (b Branch) Perspective(key string) (Perspective, bool) {
	for _, perspective := range b.Perspectives {
		if perspective.Key == key {
			return perspective, true
		}
	}
	return Perspective{}, false
}

func (p Perspective) Node(key string) (Node, bool) {
	for _, node := range p.Nodes {
		if node.Key == key {
			return node, true
		}
	}
	return Node{}, false
}
