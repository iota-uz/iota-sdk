package testharness

type TestSuite struct {
	Tests []TestCase `json:"tests"`
}

type TestCase struct {
	ID                 string   `json:"id"`
	Description        string   `json:"description"`
	DatasetID          string   `json:"dataset_id"`
	Category           string   `json:"category,omitempty"`
	Tags               []string `json:"tags,omitempty"`
	MaxDurationSeconds int      `json:"max_duration_seconds"`
	Turns              []Turn   `json:"turns"`
	UserPermissions    []string `json:"user_permissions"`
	OracleRefs         []string `json:"oracle_refs,omitempty"`
	Expect             Expect   `json:"expect"`
}

type Expect struct {
	Forbidden      bool `json:"forbidden"`
	RedirectUnauth bool `json:"redirect_unauth"`
	SSEError       bool `json:"sse_error"`
}

type Turn struct {
	Prompt            string   `json:"prompt"`
	OracleRefs        []string `json:"oracle_refs,omitempty"`
	ExpectedChecklist []string `json:"expected_checklist"`
	JudgeInstructions string   `json:"judge_instructions"`
}
