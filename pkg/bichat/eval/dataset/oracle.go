package dataset

import "github.com/iota-uz/iota-sdk/pkg/bichat/testharness"

func ToHarnessOracleFacts(facts map[string]Fact) map[string]testharness.OracleFact {
	out := make(map[string]testharness.OracleFact, len(facts))
	for key, fact := range facts {
		out[key] = testharness.OracleFact{
			Key:           fact.Key,
			Description:   fact.Description,
			ExpectedValue: fact.ExpectedValue,
			ValueType:     fact.ValueType,
			Tolerance:     fact.Tolerance,
			Normalization: fact.Normalization,
		}
	}
	return out
}
