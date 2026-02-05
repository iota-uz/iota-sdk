package testharness

type TestSuite struct {
	Tests []TestCase `json:"tests"`
}

type TestCase struct {
	ID                 string   `json:"id"`
	Description        string   `json:"description"`
	MaxDurationSeconds int      `json:"max_duration_seconds"`
	Turns              []Turn   `json:"turns"`
	UserPermissions    []string `json:"user_permissions"`
}

type Turn struct {
	Prompt            string   `json:"prompt"`
	ExpectedChecklist []string `json:"expected_checklist"`
	JudgeInstructions string   `json:"judge_instructions"`
}
