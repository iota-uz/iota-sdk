package testharness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	oaiconstant "github.com/openai/openai-go/shared/constant"
)

type OracleFact struct {
	Key           string  `json:"key"`
	Description   string  `json:"description,omitempty"`
	ExpectedValue string  `json:"expected_value"`
	ValueType     string  `json:"value_type,omitempty"`
	Tolerance     float64 `json:"tolerance,omitempty"`
	Normalization string  `json:"normalization,omitempty"`
}

type JudgeUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Cost             float64 `json:"cost"`
	Currency         string  `json:"currency,omitempty"`
	EstimatedCost    bool    `json:"estimated_cost"`
}

type JudgeVerdict struct {
	Passed          bool        `json:"passed"`
	Score           float64     `json:"score"`
	Reason          string      `json:"reason"`
	MissedFacts     []string    `json:"missed_facts,omitempty"`
	IncorrectClaims []string    `json:"incorrect_claims,omitempty"`
	Usage           *JudgeUsage `json:"usage,omitempty"`
}

type JudgeTurnInput struct {
	UserPrompt        string       `json:"user_prompt"`
	FinalAnswer       string       `json:"final_answer"`
	StreamedAnswer    string       `json:"streamed_answer"`
	SSEError          string       `json:"sse_error"`
	ExpectedChecklist []string     `json:"expected_checklist"`
	JudgeInstructions string       `json:"judge_instructions"`
	ToolCalls         []ToolCall   `json:"tool_calls"`
	OracleFacts       []OracleFact `json:"oracle_facts,omitempty"`
}

type OpenAIJudge struct {
	client openai.Client
	model  string
}

func NewOpenAIJudge(cfg Config) *OpenAIJudge {
	return &OpenAIJudge{
		client: openai.NewClient(option.WithAPIKey(cfg.OpenAIAPIKey)),
		model:  cfg.JudgeModel,
	}
}

const judgeSystemPrompt = `You are an evaluation judge for an AI analytics assistant.

Evaluate the assistant answer against:
1) Oracle facts (authoritative known answers)
2) Checklist items
3) Judge instructions

Scoring and pass/fail:
- score is between 0.0 and 1.0
- passed=true only when all critical requested facts are correct and there are no material false claims
- Any contradiction of oracle facts should fail

Normalization rules:
- Compare numeric facts after normalization when provided (for example, currency in minor units)
- Respect tolerance if provided
- Date comparisons should allow formatting differences but preserve actual date meaning

Return ONLY valid JSON with keys:
- passed (boolean)
- score (number 0..1)
- reason (short explanation)
- missed_facts (array of oracle keys not satisfied)
- incorrect_claims (array of short claim strings that contradict oracle facts)`

func (j *OpenAIJudge) Evaluate(ctx context.Context, in JudgeTurnInput) (*JudgeVerdict, error) {
	userPrompt := buildJudgeUserPrompt(in)

	maxTokens := int64(1024)
	resp, err := j.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: j.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(judgeSystemPrompt),
			openai.UserMessage(userPrompt),
		},
		// GPT-5* models require max_completion_tokens (max_tokens is not supported).
		MaxCompletionTokens: openai.Int(maxTokens),
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &openai.ResponseFormatJSONObjectParam{
				Type: oaiconstant.ValueOf[oaiconstant.JSONObject](),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("judge request failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("judge returned no choices")
	}

	content := resp.Choices[0].Message.Content
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("judge returned empty content")
	}

	verdict, err := parseJudgeVerdict([]byte(content))
	if err != nil {
		return nil, fmt.Errorf("judge verdict parse failed: %w", err)
	}
	verdict.Usage = buildJudgeUsage(j.model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	return verdict, nil
}

func buildJudgeUserPrompt(in JudgeTurnInput) string {
	var b strings.Builder
	b.WriteString("Evaluate the following turn.\\n\\n")
	b.WriteString("User prompt:\\n")
	b.WriteString(in.UserPrompt)
	b.WriteString("\\n\\n")

	if len(in.OracleFacts) > 0 {
		b.WriteString("Oracle facts (authoritative):\\n")
		for _, fact := range in.OracleFacts {
			if strings.TrimSpace(fact.Key) == "" {
				continue
			}
			line := fmt.Sprintf("- %s => %s", fact.Key, fact.ExpectedValue)
			if strings.TrimSpace(fact.ValueType) != "" {
				line += fmt.Sprintf(" | type=%s", fact.ValueType)
			}
			if fact.Tolerance > 0 {
				line += fmt.Sprintf(" | tolerance=%g", fact.Tolerance)
			}
			if strings.TrimSpace(fact.Normalization) != "" {
				line += fmt.Sprintf(" | normalization=%s", fact.Normalization)
			}
			if strings.TrimSpace(fact.Description) != "" {
				line += fmt.Sprintf(" | note=%s", fact.Description)
			}
			b.WriteString(line)
			b.WriteString("\\n")
		}
		b.WriteString("\\n")
	}

	if len(in.ExpectedChecklist) > 0 {
		b.WriteString("Expected checklist:\\n")
		for _, item := range in.ExpectedChecklist {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(item)
			b.WriteString("\\n")
		}
		b.WriteString("\\n")
	}

	if strings.TrimSpace(in.JudgeInstructions) != "" {
		b.WriteString("Judge instructions:\\n")
		b.WriteString(in.JudgeInstructions)
		b.WriteString("\\n\\n")
	}

	if strings.TrimSpace(in.SSEError) != "" {
		b.WriteString("SSE error payload:\\n")
		b.WriteString(in.SSEError)
		b.WriteString("\\n\\n")
	}

	if len(in.ToolCalls) > 0 {
		b.WriteString("Tool calls (sequence):\\n")
		for i, tc := range in.ToolCalls {
			b.WriteString(fmt.Sprintf("%d. %s\\n", i+1, tc.Name))
		}
		b.WriteString("\\n")
	}

	b.WriteString("Final answer (authoritative):\\n")
	b.WriteString(in.FinalAnswer)
	b.WriteString("\\n\\n")

	if strings.TrimSpace(in.FinalAnswer) == "" && strings.TrimSpace(in.StreamedAnswer) != "" {
		b.WriteString("Streamed answer (fallback):\\n")
		b.WriteString(in.StreamedAnswer)
		b.WriteString("\\n\\n")
	}

	b.WriteString(`Respond with JSON: {"passed":true,"score":1,"reason":"brief","missed_facts":[],"incorrect_claims":[]}`)
	return b.String()
}

func buildJudgeUsage(model string, promptTokens, completionTokens, totalTokens int64) *JudgeUsage {
	if promptTokens <= 0 && completionTokens <= 0 && totalTokens <= 0 {
		return nil
	}

	cost, currency, estimated := estimateJudgeCost(model, promptTokens, completionTokens)
	return &JudgeUsage{
		PromptTokens:     int(promptTokens),
		CompletionTokens: int(completionTokens),
		TotalTokens:      int(totalTokens),
		Cost:             cost,
		Currency:         currency,
		EstimatedCost:    estimated,
	}
}

type judgePricing struct {
	Currency    string
	InputPer1M  float64
	OutputPer1M float64
}

func estimateJudgeCost(model string, promptTokens, completionTokens int64) (float64, string, bool) {
	pricing := map[string]judgePricing{
		"gpt-5.2-2025-12-11": {Currency: "USD", InputPer1M: 1.75, OutputPer1M: 14.00},
		"gpt-5-mini":         {Currency: "USD", InputPer1M: 1.75, OutputPer1M: 14.00},
		"gpt-5-nano":         {Currency: "USD", InputPer1M: 1.75, OutputPer1M: 14.00},
		"gpt-4o":             {Currency: "USD", InputPer1M: 2.50, OutputPer1M: 10.00},
		"gpt-4o-mini":        {Currency: "USD", InputPer1M: 0.150, OutputPer1M: 0.600},
		"gpt-4-turbo":        {Currency: "USD", InputPer1M: 10.00, OutputPer1M: 30.00},
		"gpt-4":              {Currency: "USD", InputPer1M: 30.00, OutputPer1M: 60.00},
	}

	model = strings.TrimSpace(strings.ToLower(model))
	p, ok := pricing[model]
	if !ok {
		p = judgePricing{Currency: "USD", InputPer1M: 1.75, OutputPer1M: 14.00}
	}

	inputCost := (float64(promptTokens) / 1_000_000) * p.InputPer1M
	outputCost := (float64(completionTokens) / 1_000_000) * p.OutputPer1M
	return inputCost + outputCost, p.Currency, true
}

func parseJudgeVerdict(data []byte) (*JudgeVerdict, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var v JudgeVerdict
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if v.Score < 0 || v.Score > 1 {
		return nil, errors.New("score must be between 0 and 1")
	}
	if strings.TrimSpace(v.Reason) == "" {
		return nil, errors.New("reason is required")
	}
	return &v, nil
}
