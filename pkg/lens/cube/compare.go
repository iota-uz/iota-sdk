package cube

import (
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

const comparisonSuffix = "_comparison"

type comparisonConfig struct {
	Enabled  bool
	Variable string
	Anchor   string
}

func resolveComparison(spec CubeSpec, ctx DrillContext) comparisonConfig {
	for _, variable := range spec.Variables {
		if variable.Kind != lens.VariableCompare {
			continue
		}
		mode := lens.ResolveCompareMode(variable, ctx.Request)
		switch mode {
		case lens.ComparePreviousPeriod, lens.CompareYearAgo, lens.CompareCustom:
			return comparisonConfig{Enabled: true, Variable: variable.Name, Anchor: variable.CompareTo}
		case lens.CompareOff:
			return comparisonConfig{}
		default:
			return comparisonConfig{}
		}
	}
	return comparisonConfig{}
}

func comparisonDatasetName(name string) string { return name + comparisonSuffix }

func comparedDatasetName(name string) string { return name + "_compared" }

func comparisonField(name string) string { return "previous_" + name }

func deltaField(name string) string { return name + "_delta" }

func deltaRatioField(name string) string { return name + "_delta_ratio" }

func deltaPercentField(name string) string { return name + "_delta_percent" }

func cloneComparisonGraph(
	datasets []lens.DatasetSpec,
	comparison comparisonConfig,
	terminalFields map[string][]string,
) []lens.DatasetSpec {
	names := make(map[string]string, len(datasets))
	for _, dataset := range datasets {
		names[dataset.Name] = comparisonDatasetName(dataset.Name)
	}
	cloned := make([]lens.DatasetSpec, 0, len(datasets))
	for _, source := range datasets {
		dataset := source
		dataset.Name = names[source.Name]
		dataset.DependsOn = append([]string(nil), source.DependsOn...)
		for index, dependency := range dataset.DependsOn {
			if renamed, ok := names[dependency]; ok {
				dataset.DependsOn[index] = renamed
			}
		}
		dataset.Transforms = append([]transform.Spec(nil), source.Transforms...)
		if source.Query != nil {
			query := *source.Query
			query.Params = cloneParamValues(source.Query.Params)
			for key, param := range query.Params {
				if param.Variable == comparison.Anchor {
					param.Variable = comparison.Variable
					query.Params[key] = param
				}
			}
			dataset.Query = &query
			dataset.TimeRangeVariable = comparison.Variable
		}
		if fields := terminalFields[source.Name]; len(fields) > 0 {
			aliases := make(map[string]string, len(fields))
			for _, field := range fields {
				aliases[field] = comparisonField(field)
			}
			dataset.Transforms = append(dataset.Transforms, transform.Spec{Kind: transform.KindRename, Aliases: aliases})
		}
		cloned = append(cloned, dataset)
	}
	return cloned
}

func comparisonJoin(terminal string, fields, on []string) lens.DatasetSpec {
	other := comparisonDatasetName(terminal)
	transforms := []transform.Spec{{
		Kind: transform.KindJoin,
		Join: &transform.JoinConfig{Other: other, On: append([]string(nil), on...), How: "left"},
	}}
	for _, field := range fields {
		transforms = append(transforms,
			transform.Spec{Kind: transform.KindFormula, Formula: &transform.Formula{
				As: deltaField(field), Op: "-", Left: field, Right: comparisonField(field),
			}},
			transform.Spec{Kind: transform.KindFormula, Formula: &transform.Formula{
				As: deltaRatioField(field), Op: "/", Left: deltaField(field), Right: comparisonField(field),
			}},
			transform.Spec{Kind: transform.KindFormula, Formula: &transform.Formula{
				As: deltaPercentField(field), Op: "*", Left: deltaRatioField(field), RightValue: 100,
			}},
		)
	}
	return lens.DatasetSpec{
		Name: comparedDatasetName(terminal), Kind: lens.DatasetKindTransform,
		DependsOn: []string{terminal, other}, Transforms: transforms,
	}
}
