package cube

import (
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/comparison"
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

func cloneComparisonGraph(
	datasets []lens.DatasetSpec,
	config comparisonConfig,
	terminalFields map[string][]string,
	topNRankKeys map[string][]string,
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
		for index := range dataset.Transforms {
			topN := dataset.Transforms[index].TopN
			keyFields := topNRankKeys[source.Name]
			if topN == nil || len(keyFields) == 0 {
				continue
			}
			clonedTopN := *topN
			clonedTopN.RankByDataset = source.Name
			clonedTopN.KeyFields = append([]string(nil), keyFields...)
			dataset.Transforms[index].TopN = &clonedTopN
			dataset.DependsOn = append(dataset.DependsOn, source.Name)
		}
		if source.Query != nil {
			query := *source.Query
			query.Params = cloneParamValues(source.Query.Params)
			for key, param := range query.Params {
				if param.Variable == config.Anchor {
					param.Variable = config.Variable
					query.Params[key] = param
				}
			}
			dataset.Query = &query
			dataset.TimeRangeVariable = config.Variable
		}
		if fields := terminalFields[source.Name]; len(fields) > 0 {
			aliases := make(map[string]string, len(fields))
			for _, field := range fields {
				aliases[field] = comparison.PreviousField(field)
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
				As: comparison.DeltaField(field), Op: "-", Left: field, Right: comparison.PreviousField(field),
			}},
			transform.Spec{Kind: transform.KindFormula, Formula: &transform.Formula{
				As: comparison.DeltaRatioField(field), Op: "/", Left: comparison.DeltaField(field), Right: comparison.PreviousField(field), AbsoluteRight: true,
			}},
			transform.Spec{Kind: transform.KindFormula, Formula: &transform.Formula{
				As: comparison.DeltaPercentField(field), Op: "*", Left: comparison.DeltaRatioField(field), RightValue: 100,
			}},
		)
	}
	return lens.DatasetSpec{
		Name: comparedDatasetName(terminal), Kind: lens.DatasetKindTransform,
		DependsOn: []string{terminal, other}, Transforms: transforms,
	}
}
