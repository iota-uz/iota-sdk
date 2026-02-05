package testharness

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseJudgeVerdict_Strict(t *testing.T) {
	t.Parallel()

	_, err := parseJudgeVerdict([]byte(`{"passed":true,"reason":"ok","efficiency_score":3,"efficiency_notes":"fine","extra":1}`))
	require.Error(t, err)

	_, err = parseJudgeVerdict([]byte(`{"passed":true,"reason":"ok","efficiency_score":0,"efficiency_notes":"fine"}`))
	require.Error(t, err)

	_, err = parseJudgeVerdict([]byte(`{"passed":true,"reason":"one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen","efficiency_score":3,"efficiency_notes":"fine"}`))
	require.Error(t, err)

	_, err = parseJudgeVerdict([]byte(`{"passed":true,"reason":"ok","efficiency_score":3,"efficiency_notes":""}`))
	require.Error(t, err)

	v, err := parseJudgeVerdict([]byte(`{"passed":true,"reason":"looks good","efficiency_score":5,"efficiency_notes":"minimal tools used"}`))
	require.NoError(t, err)
	require.True(t, v.Passed)
	require.Equal(t, 5, v.EfficiencyScore)
}
