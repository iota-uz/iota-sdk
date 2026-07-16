package apex

import (
	"fmt"
	"strings"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

// buildDrillTreeJS delegates a chart click to the shared, renderer-agnostic
// DrillTree state machine. The handler consumes only configured keyed branches;
// every other click is passed to the legacy circular handler or panel action.
func buildDrillTreeJS(
	spec *panel.DrillTree,
	fr *frame.Frame,
	fields panel.FieldMapping,
	formatterSpec *format.Spec,
	locale string,
	rootLabel string,
	panelResult *runtime.PanelResult,
	fallback templ.JSExpression,
) templ.JSExpression {
	configJS, ok := drillTreeConfigJS(spec, fr, fields, formatterSpec, locale, rootLabel, panelResult)
	if !ok {
		return fallback
	}
	fallbackJS := "null"
	hasFallbackAction := false
	if fallback != "" {
		fallbackJS = "(" + string(fallback) + ")"
		hasFallbackAction = true
	}
	return templ.JSExpression(fmt.Sprintf(`function(event, chartContext, opts) {
		var cfg = %s;
		cfg.hasFallbackAction = %t;
		if (window.__lensDrillTreeClick && window.__lensDrillTreeClick(event, chartContext, opts, cfg)) {
			return;
		}
		var fallback = %s;
		if (fallback) {
			fallback(event, chartContext, opts);
		}
	}`, configJS, hasFallbackAction, fallbackJS))
}

func buildDrillTreeMountJS(
	spec *panel.DrillTree,
	fr *frame.Frame,
	fields panel.FieldMapping,
	formatterSpec *format.Spec,
	locale string,
	rootLabel string,
	panelResult *runtime.PanelResult,
	hasFallbackAction bool,
) templ.JSExpression {
	configJS, ok := drillTreeConfigJS(spec, fr, fields, formatterSpec, locale, rootLabel, panelResult)
	if !ok {
		return ""
	}
	return templ.JSExpression(fmt.Sprintf(`function(chartContext) {
		var cfg = %s;
		cfg.hasFallbackAction = %t;
		if (window.__lensDrillTreeMount) {
			window.__lensDrillTreeMount(chartContext, cfg);
		}
	}`, configJS, hasFallbackAction))
}

func drillTreeConfigJS(
	spec *panel.DrillTree,
	fr *frame.Frame,
	fields panel.FieldMapping,
	formatterSpec *format.Spec,
	locale string,
	rootLabel string,
	panelResult *runtime.PanelResult,
) (string, bool) {
	if spec == nil || fr == nil || fields.ID.Empty() {
		return "", false
	}
	locale = normalizedChartLocale(locale)
	rootKeys, rootValuesFormatted, ok := drillTreeRootValues(fr, fields.ID, fields.Value, formatterSpec, locale)
	if !ok {
		return "", false
	}
	branches := make([]drillTreeBranchConfig, 0, len(spec.Branches))
	for _, branch := range spec.Branches {
		if strings.TrimSpace(branch.TriggerKey) == "" || strings.TrimSpace(branch.Label) == "" {
			continue
		}
		children, total := drillTreeNodesConfig(branch.Children, formatterSpec, locale, panelResult)
		if len(children) == 0 {
			continue
		}
		branches = append(branches, drillTreeBranchConfig{
			TriggerKey:     branch.TriggerKey,
			Label:          branch.Label,
			Children:       children,
			Total:          total,
			TotalFormatted: format.Apply(formatterSpec, total, locale, ""),
		})
	}
	if len(branches) == 0 {
		return "", false
	}
	return mustJSONJS(drillTreeConfig{
		RootKeys:            rootKeys,
		RootValuesFormatted: rootValuesFormatted,
		RootLabel:           rootLabel,
		BackLabel:           drillBackLabel(locale),
		Branches:            branches,
	}), true
}

func drillTreeRootValues(fr *frame.Frame, idField, valueField panel.FieldRef, formatterSpec *format.Spec, locale string) ([]string, []string, bool) {
	rows := fr.Rows()
	keys := make([]string, len(rows))
	formatted := make([]string, len(rows))
	for i, row := range rows {
		value, exists := row[idField.Name()]
		key, isString := value.(string)
		if !exists || !isString || strings.TrimSpace(key) == "" {
			return nil, nil, false
		}
		keys[i] = key
		formatted[i] = format.Apply(formatterSpec, numericValue(row[valueField.Name()]), locale, "")
	}
	return keys, formatted, len(keys) > 0
}

func drillTreeNodesConfig(
	nodes []panel.DrillNode,
	formatterSpec *format.Spec,
	locale string,
	panelResult *runtime.PanelResult,
) ([]drillTreeNodeConfig, float64) {
	configured := make([]drillTreeNodeConfig, 0, len(nodes))
	total := 0.0
	for _, node := range nodes {
		if strings.TrimSpace(node.Key) == "" || strings.TrimSpace(node.Label) == "" || node.Value < 0 {
			continue
		}
		children, childTotal := drillTreeNodesConfig(node.Children, formatterSpec, locale, panelResult)
		configuredNode := drillTreeNodeConfig{
			Key:            node.Key,
			Label:          node.Label,
			Value:          node.Value,
			ValueFormatted: format.Apply(formatterSpec, node.Value, locale, ""),
			Color:          node.Color,
			Action:         drillTreeAction(node.Action, panelResult),
			Children:       children,
		}
		if len(children) > 0 {
			configuredNode.Total = childTotal
			configuredNode.TotalFormatted = format.Apply(formatterSpec, childTotal, locale, "")
		}
		configured = append(configured, configuredNode)
		total += node.Value
	}
	return configured, total
}

func drillTreeAction(spec *action.Spec, panelResult *runtime.PanelResult) *drillTreeActionConfig {
	if spec == nil {
		return nil
	}
	switch spec.Kind {
	case action.KindNavigate, action.KindHtmxSwap, action.KindEmitEvent:
	default:
		return nil
	}
	variables := map[string]any(nil)
	baseQuery := map[string][]string(nil)
	if panelResult != nil {
		variables = panelResult.Variables
		if spec.PreserveQuery {
			baseQuery = copiedQueryMap(panelResult.Request)
		}
	}

	urlValue := spec.URL
	if spec.URLSource != nil {
		resolved, ok := action.ResolveValue(*spec.URLSource, nil, variables)
		if !ok {
			return nil
		}
		urlValue = fmt.Sprint(resolved)
	}
	if spec.Kind != action.KindEmitEvent {
		var safe bool
		urlValue, safe = action.SafeRelativeURL(urlValue)
		if !safe {
			return nil
		}
	}

	params := make([]drillTreeActionParam, 0, len(spec.Params))
	for _, param := range spec.Params {
		value, ok := action.ResolveValue(param.Source, nil, variables)
		if !ok {
			return nil
		}
		params = append(params, drillTreeActionParam{Name: param.Name, Value: value})
	}
	payload := make(map[string]any, len(spec.Payload))
	for name, source := range spec.Payload {
		value, ok := action.ResolveValue(source, nil, variables)
		if !ok {
			return nil
		}
		payload[name] = value
	}
	method := spec.Method
	if method == "" {
		method = "GET"
	}
	return &drillTreeActionConfig{
		Kind:          string(spec.Kind),
		Method:        method,
		URL:           urlValue,
		Target:        spec.Target,
		Event:         spec.Event,
		Params:        params,
		Payload:       payload,
		BaseQuery:     baseQuery,
		PreserveQuery: spec.PreserveQuery,
	}
}

type drillTreeConfig struct {
	RootKeys            []string                `json:"rootKeys"`
	RootValuesFormatted []string                `json:"rootValuesFormatted"`
	RootLabel           string                  `json:"rootLabel,omitempty"`
	BackLabel           string                  `json:"backLabel"`
	Branches            []drillTreeBranchConfig `json:"branches"`
}

type drillTreeBranchConfig struct {
	TriggerKey     string                `json:"triggerKey"`
	Label          string                `json:"label"`
	Children       []drillTreeNodeConfig `json:"children"`
	Total          float64               `json:"total"`
	TotalFormatted string                `json:"totalFormatted"`
}

type drillTreeNodeConfig struct {
	Key            string                 `json:"key"`
	Label          string                 `json:"label"`
	Value          float64                `json:"value"`
	ValueFormatted string                 `json:"valueFormatted"`
	Color          string                 `json:"color,omitempty"`
	Action         *drillTreeActionConfig `json:"action,omitempty"`
	Children       []drillTreeNodeConfig  `json:"children,omitempty"`
	Total          float64                `json:"total,omitempty"`
	TotalFormatted string                 `json:"totalFormatted,omitempty"`
}

type drillTreeActionConfig struct {
	Kind          string                 `json:"kind"`
	Method        string                 `json:"method,omitempty"`
	URL           string                 `json:"url,omitempty"`
	Target        string                 `json:"target,omitempty"`
	Event         string                 `json:"event,omitempty"`
	Params        []drillTreeActionParam `json:"params,omitempty"`
	Payload       map[string]any         `json:"payload,omitempty"`
	BaseQuery     map[string][]string    `json:"baseQuery,omitempty"`
	PreserveQuery bool                   `json:"preserveQuery,omitempty"`
}

type drillTreeActionParam struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}
