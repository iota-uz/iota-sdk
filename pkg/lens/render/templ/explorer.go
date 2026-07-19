package templ

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

type explorerClientConfig struct {
	ID           string                 `json:"id"`
	HostPanelID  string                 `json:"hostPanelId"`
	ExpandedSpan int                    `json:"expandedSpan,omitempty"`
	Branches     []explorerClientBranch `json:"branches"`
	Text         explorerClientText     `json:"text"`
}

func explorationFragmentResult(result *runtime.ExplorationResult) (panel.Spec, *runtime.Result, bool) {
	if result == nil || result.Panel == nil {
		return panel.Spec{}, nil, false
	}
	panelSpec := result.Panel.Panel
	if len(explorationResultEdges(result)) > 0 && panelSpec.Fields.ID.Name() != "" {
		panelSpec.Action = &action.Spec{
			Kind:  action.KindEmitEvent,
			Event: "lens:explorer-point",
			Payload: map[string]action.ValueSource{
				"explorerId": action.LiteralValue(result.Explorer.ID),
				"pointKey":   action.FieldValue(panelSpec.Fields.ID.Name()),
			},
		}
	}
	panelResult := *result.Panel
	panelResult.Panel = panelSpec
	dashboardResult := &runtime.Result{Panels: map[string]*runtime.PanelResult{panelSpec.ID: &panelResult}}
	return panelSpec, dashboardResult, true
}

type explorerClientBranch struct {
	Key                string                      `json:"key"`
	Label              string                      `json:"label"`
	DefaultPerspective string                      `json:"defaultPerspective"`
	Perspectives       []explorerClientPerspective `json:"perspectives"`
}

type explorerClientPerspective struct {
	Key      string               `json:"key"`
	Label    string               `json:"label"`
	RootNode string               `json:"rootNode"`
	Nodes    []explorerClientNode `json:"nodes"`
}

type explorerClientNode struct {
	Key            string               `json:"key"`
	Label          string               `json:"label"`
	Eager          bool                 `json:"eager,omitempty"`
	Load           *explore.LoadSpec    `json:"load,omitempty"`
	Edges          []explorerClientEdge `json:"edges,omitempty"`
	DynamicEdges   bool                 `json:"dynamicEdges,omitempty"`
	DynamicTargets []string             `json:"dynamicTargets,omitempty"`
}

type explorerClientEdge struct {
	PointKey string                `json:"pointKey"`
	ToNode   string                `json:"toNode,omitempty"`
	Action   *explorerClientAction `json:"action,omitempty"`
}

type explorerClientAction struct {
	Kind          string         `json:"kind"`
	URL           string         `json:"url,omitempty"`
	Method        string         `json:"method,omitempty"`
	Target        string         `json:"target,omitempty"`
	Event         string         `json:"event,omitempty"`
	PreserveQuery bool           `json:"preserveQuery,omitempty"`
	Params        map[string]any `json:"params,omitempty"`
	Payload       map[string]any `json:"payload,omitempty"`
}

type explorerClientText struct {
	Back        string `json:"back"`
	Home        string `json:"home"`
	Expand      string `json:"expand"`
	Collapse    string `json:"collapse"`
	Loading     string `json:"loading"`
	Unavailable string `json:"unavailable"`
	Retry       string `json:"retry"`
	Error       string `json:"error"`
}

func explorerForHost(items []explore.Spec, panelID string) *explore.Spec {
	for i := range items {
		if items[i].HostPanelID == panelID {
			return &items[i]
		}
	}
	return nil
}

func explorerClientConfigJSON(ctxText explorerClientText, spec explore.Spec) string {
	config := explorerClientConfig{
		ID:           spec.ID,
		HostPanelID:  spec.HostPanelID,
		ExpandedSpan: spec.ExpandedSpan,
		Text:         ctxText,
	}
	for _, branch := range spec.Branches {
		clientBranch := explorerClientBranch{
			Key:                branch.Key,
			Label:              branch.Label,
			DefaultPerspective: branch.DefaultPerspective,
		}
		for _, perspective := range branch.Perspectives {
			clientPerspective := explorerClientPerspective{
				Key: perspective.Key, Label: perspective.Label, RootNode: perspective.RootNode,
			}
			for _, node := range perspective.Nodes {
				clientNode := explorerClientNode{Key: node.Key, Label: node.Label, Eager: node.Panel != nil, Load: node.Load, DynamicEdges: node.DynamicEdges, DynamicTargets: append([]string(nil), node.DynamicTargets...)}
				for _, edge := range node.Edges {
					clientEdge := explorerClientEdge{PointKey: edge.PointKey, ToNode: edge.ToNode}
					if edge.Action != nil {
						clientEdge.Action = clientAction(edge.Action)
					}
					clientNode.Edges = append(clientNode.Edges, clientEdge)
				}
				clientPerspective.Nodes = append(clientPerspective.Nodes, clientNode)
			}
			clientBranch.Perspectives = append(clientBranch.Perspectives, clientPerspective)
		}
		config.Branches = append(config.Branches, clientBranch)
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func explorationFragmentEdgesJSON(result *runtime.ExplorationResult) string {
	if result == nil {
		return "[]"
	}
	resolved := explorationResultEdges(result)
	edges := make([]explorerClientEdge, 0, len(resolved))
	for _, edge := range resolved {
		clientEdge := explorerClientEdge{PointKey: edge.PointKey, ToNode: edge.ToNode}
		if edge.Action != nil {
			clientEdge.Action = clientAction(edge.Action)
		}
		edges = append(edges, clientEdge)
	}
	encoded, err := json.Marshal(edges)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func explorationResultEdges(result *runtime.ExplorationResult) []explore.Edge {
	if result == nil || len(result.Edges) > 0 {
		if result == nil {
			return nil
		}
		return result.Edges
	}
	if len(result.Path) == 0 {
		return nil
	}
	node, ok := result.View.Node(result.Path[len(result.Path)-1])
	if !ok {
		return nil
	}
	return node.Edges
}

func explorerNodeCacheKey(branchKey, perspectiveKey, nodeKey string) string {
	return branchKey + "/" + perspectiveKey + "/" + nodeKey
}

func explorerNodePanel(explorerID string, node explore.Node) panel.Spec {
	if node.Panel == nil {
		return panel.Spec{}
	}
	panelSpec := *node.Panel
	if len(node.Edges) == 0 || panelSpec.Fields.ID.Name() == "" {
		return panelSpec
	}
	panelSpec.Action = &action.Spec{
		Kind:  action.KindEmitEvent,
		Event: "lens:explorer-point",
		Payload: map[string]action.ValueSource{
			"explorerId": action.LiteralValue(explorerID),
			"pointKey":   action.FieldValue(panelSpec.Fields.ID.Name()),
		},
	}
	return panelSpec
}

func clientAction(spec *action.Spec) *explorerClientAction {
	if spec == nil {
		return nil
	}
	method := strings.ToUpper(strings.TrimSpace(spec.Method))
	if method == "" {
		method = "GET"
	}
	client := &explorerClientAction{
		Kind: string(spec.Kind), URL: spec.URL, Method: method, Target: spec.Target, Event: spec.Event, PreserveQuery: spec.PreserveQuery,
	}
	for _, param := range spec.Params {
		if param.Source.Kind != action.SourceLiteral {
			continue
		}
		if client.Params == nil {
			client.Params = make(map[string]any)
		}
		client.Params[param.Name] = param.Source.Value
	}
	for key, source := range spec.Payload {
		if source.Kind != action.SourceLiteral {
			continue
		}
		if client.Payload == nil {
			client.Payload = make(map[string]any)
		}
		client.Payload[key] = source.Value
	}
	return client
}

func localizedExplorerText(ctx context.Context) explorerClientText {
	return explorerClientText{
		Back:        translatedOr(ctx, "Lens.Explorer.Back", translate(ctx, "Lens.Drill.Back"), "Back"),
		Home:        translatedOr(ctx, "Lens.Explorer.Home", "Back to start"),
		Expand:      translatedOr(ctx, "Lens.Explorer.Expand", translate(ctx, "Chart.ExpandToFullScreen"), "Expand chart"),
		Collapse:    translatedOr(ctx, "Lens.Explorer.Collapse", translate(ctx, "Chart.CloseFullScreen"), "Close fullscreen"),
		Loading:     translatedOr(ctx, "Lens.Explorer.Loading", translate(ctx, "Lens.Chart.Loading"), "Loading…"),
		Unavailable: translatedOr(ctx, "Lens.Explorer.Empty", translate(ctx, "Lens.Empty._Description"), "This view has no data."),
		Retry:       translatedOr(ctx, "Lens.Explorer.Retry", "Retry"),
		Error:       translatedOr(ctx, "Lens.Explorer.Error", translate(ctx, "Lens.Error._Description"), "Unable to load this view."),
	}
}

func translatedOr(ctx context.Context, key string, fallbacks ...string) string {
	for _, value := range append([]string{translate(ctx, key)}, fallbacks...) {
		value = strings.TrimSpace(value)
		if value != "" && value != key && !strings.HasPrefix(value, "Lens.") {
			return value
		}
	}
	return ""
}
