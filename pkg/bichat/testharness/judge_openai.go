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

type JudgeVerdict struct {
	Passed          bool   `json:"passed"`
	Reason          string `json:"reason"`
	EfficiencyScore int    `json:"efficiency_score"`
	EfficiencyNotes string `json:"efficiency_notes"`
}

type JudgeTurnInput struct {
	UserPrompt        string     `json:"user_prompt"`
	FinalAnswer       string     `json:"final_answer"`
	StreamedAnswer    string     `json:"streamed_answer"`
	SSEError          string     `json:"sse_error"`
	ExpectedChecklist []string   `json:"expected_checklist"`
	JudgeInstructions string     `json:"judge_instructions"`
	ToolCalls         []ToolCall `json:"tool_calls"`
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

const judgeSystemPrompt = `You are an evaluation judge for an AI chat system. Evaluate BOTH correctness AND efficiency.

CORRECTNESS (passed: true/false)
PASS if:
- The assistant satisfies the checklist items relevant to the prompt.
- The assistant provides a substantive answer or a justified refusal.

FAIL if:
- The assistant is unrelated, empty, or clearly incomplete.
- The assistant ignores key requirements from the checklist/instructions.

EFFICIENCY (efficiency_score: 1-5)
5 = Minimal, direct, no redundant tool usage.
3 = Some redundancy or one retry.
1 = Excessive redundancy or confused tool usage.

Constraints:
- "reason" must be 15 words or fewer.
- "efficiency_notes" must be 20 words or fewer.

Return ONLY valid JSON with keys: passed, reason, efficiency_score, efficiency_notes.`

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
	return verdict, nil
}

func buildJudgeUserPrompt(in JudgeTurnInput) string {
	var b strings.Builder
	b.WriteString("Evaluate the following turn.\n\n")
	b.WriteString("User prompt:\n")
	b.WriteString(in.UserPrompt)
	b.WriteString("\n\n")

	if len(in.ExpectedChecklist) > 0 {
		b.WriteString("Expected checklist:\n")
		for _, item := range in.ExpectedChecklist {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(item)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if strings.TrimSpace(in.JudgeInstructions) != "" {
		b.WriteString("Judge instructions:\n")
		b.WriteString(in.JudgeInstructions)
		b.WriteString("\n\n")
	}

	if strings.TrimSpace(in.SSEError) != "" {
		b.WriteString("SSE error payload:\n")
		b.WriteString(in.SSEError)
		b.WriteString("\n\n")
	}

	if len(in.ToolCalls) > 0 {
		b.WriteString("Tool calls (sequence):\n")
		for i, tc := range in.ToolCalls {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, tc.Name))
		}
		b.WriteString("\n")
	}

	b.WriteString("Final answer (authoritative):\n")
	b.WriteString(in.FinalAnswer)
	b.WriteString("\n\n")

	if strings.TrimSpace(in.FinalAnswer) == "" && strings.TrimSpace(in.StreamedAnswer) != "" {
		b.WriteString("Streamed answer (fallback):\n")
		b.WriteString(in.StreamedAnswer)
		b.WriteString("\n\n")
	}

	b.WriteString(`Respond with JSON: {"passed":true,"reason":"<=15 words","efficiency_score":1,"efficiency_notes":"<=20 words"}`)
	return b.String()
}

func parseJudgeVerdict(data []byte) (*JudgeVerdict, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var v JudgeVerdict
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if v.EfficiencyScore < 1 || v.EfficiencyScore > 5 {
		return nil, errors.New("efficiency_score must be 1-5")
	}
	if countWords(v.Reason) > 15 {
		return nil, errors.New("reason exceeds 15 words")
	}
	if countWords(v.EfficiencyNotes) > 20 {
		return nil, errors.New("efficiency_notes exceeds 20 words")
	}
	if strings.TrimSpace(v.Reason) == "" || strings.TrimSpace(v.EfficiencyNotes) == "" {
		return nil, errors.New("reason and efficiency_notes are required")
	}
	return &v, nil
}

func countWords(s string) int {
	return len(strings.Fields(strings.TrimSpace(s)))
}
