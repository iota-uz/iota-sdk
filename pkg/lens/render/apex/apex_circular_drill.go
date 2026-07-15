package apex

import (
	"fmt"
	"strings"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
)

// buildCircularDrillHierarchyJS combines the in-place circular drill handler
// with the panel's ordinary action. The shared handler returns true only when
// it consumed the click (the aggregate slice or a detail slice), so all other
// top-level slices retain their existing drawer/navigation behavior.
func buildCircularDrillHierarchyJS(spec *panel.CircularDrillHierarchy, formatterSpec *format.Spec, locale string, actionSpec *action.Spec, fallback templ.JSExpression) templ.JSExpression {
	configJS, ok := circularDrillHierarchyConfigJS(spec, formatterSpec, locale, actionSpec)
	if !ok {
		return fallback
	}
	fallbackJS := "null"
	if fallback != "" {
		fallbackJS = "(" + string(fallback) + ")"
	}
	return templ.JSExpression(fmt.Sprintf(`function(event, chartContext, opts) {
		var cfg = %s;
		if (window.__lensCircularDrillClick && window.__lensCircularDrillClick(event, chartContext, opts, cfg)) {
			return;
		}
		var fallback = %s;
		if (fallback) {
			fallback(event, chartContext, opts);
		}
	}`, configJS, fallbackJS))
}

func buildCircularDrillHierarchyMountJS(spec *panel.CircularDrillHierarchy, formatterSpec *format.Spec, locale string, actionSpec *action.Spec) templ.JSExpression {
	configJS, ok := circularDrillHierarchyConfigJS(spec, formatterSpec, locale, actionSpec)
	if !ok {
		return ""
	}
	return templ.JSExpression(fmt.Sprintf(`function(chartContext) {
		var cfg = %s;
		if (window.__lensCircularDrillMount) {
			window.__lensCircularDrillMount(chartContext, cfg);
		}
	}`, configJS))
}

func circularDrillHierarchyConfigJS(spec *panel.CircularDrillHierarchy, formatterSpec *format.Spec, locale string, actionSpec *action.Spec) (string, bool) {
	if spec == nil {
		return "", false
	}
	locale = normalizedChartLocale(locale)
	branches := make([]panel.CircularDrillBranch, 0, len(spec.Branches)+1)
	branches = append(branches, spec.Branches...)
	if strings.TrimSpace(spec.TriggerLabel) != "" && len(spec.Detail) > 0 {
		branches = append(branches, panel.CircularDrillBranch{
			TriggerLabel: spec.TriggerLabel,
			Detail:       spec.Detail,
		})
	}
	configuredBranches := make([]circularDrillBranchConfig, 0, len(branches))
	for _, branch := range branches {
		if strings.TrimSpace(branch.TriggerLabel) == "" {
			continue
		}
		detail, total := circularDrillSlicesConfig(branch.Detail, formatterSpec, locale)
		if len(detail) == 0 {
			continue
		}
		configuredBranches = append(configuredBranches, circularDrillBranchConfig{
			TriggerLabel: branch.TriggerLabel,
			Detail:       detail,
			DetailTotal:  format.Apply(formatterSpec, total, locale, ""),
		})
	}
	if len(configuredBranches) == 0 {
		return "", false
	}
	cfg := circularDrillConfig{
		Branches:  configuredBranches,
		BackLabel: drillBackLabel(locale),
	}
	if actionSpec != nil {
		cfg.ActionKind = string(actionSpec.Kind)
		cfg.ActionMethod = actionSpec.Method
		cfg.ActionTarget = actionSpec.Target
	}
	return mustJSONJS(cfg), true
}

func circularDrillSlicesConfig(items []panel.CircularDrillSlice, formatterSpec *format.Spec, locale string) ([]circularDrillSliceConfig, float64) {
	detail := make([]circularDrillSliceConfig, 0, len(items))
	total := 0.0
	for _, item := range items {
		if strings.TrimSpace(item.Label) == "" || item.Value < 0 {
			continue
		}
		children, childTotal := circularDrillSlicesConfig(item.Detail, formatterSpec, locale)
		configured := circularDrillSliceConfig{
			Label:     item.Label,
			Value:     item.Value,
			Color:     item.Color,
			ActionURL: item.ActionURL,
			Detail:    children,
		}
		if len(children) > 0 {
			configured.DetailTotal = format.Apply(formatterSpec, childTotal, locale, "")
		}
		total += item.Value
		detail = append(detail, configured)
	}
	return detail, total
}

type circularDrillConfig struct {
	Branches     []circularDrillBranchConfig `json:"branches"`
	BackLabel    string                      `json:"backLabel"`
	ActionKind   string                      `json:"actionKind,omitempty"`
	ActionMethod string                      `json:"actionMethod,omitempty"`
	ActionTarget string                      `json:"actionTarget,omitempty"`
}

type circularDrillBranchConfig struct {
	TriggerLabel string                     `json:"triggerLabel"`
	Detail       []circularDrillSliceConfig `json:"detail"`
	DetailTotal  string                     `json:"detailTotal"`
}

type circularDrillSliceConfig struct {
	Label       string                     `json:"label"`
	Value       float64                    `json:"value"`
	Color       string                     `json:"color,omitempty"`
	ActionURL   string                     `json:"actionUrl,omitempty"`
	Detail      []circularDrillSliceConfig `json:"detail,omitempty"`
	DetailTotal string                     `json:"detailTotal,omitempty"`
}
