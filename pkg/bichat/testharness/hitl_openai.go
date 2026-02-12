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

type HITLAnswerInput struct {
	CaseID          string
	CaseDescription string
	UserPrompt      string
	Questions       []HITLQuestion
	OracleFacts     []OracleFact
}

type HITLAnswerResult struct {
	Answers map[string]string `json:"answers"`
	Usage   *JudgeUsage       `json:"usage,omitempty"`
}

type OpenAIHITLResponder struct {
	client openai.Client
	model  string
}

func NewOpenAIHITLResponder(cfg Config) *OpenAIHITLResponder {
	return &OpenAIHITLResponder{
		client: openai.NewClient(option.WithAPIKey(cfg.OpenAIAPIKey)),
		model:  cfg.HITLModel,
	}
}

const hitlSystemPrompt = `You are an automatic responder for ask_user_question interrupts in an analytics evaluation harness.

Your job:
- Choose options that best help the analytics assistant answer the user's prompt correctly.
- Choose exactly one option ID per question, even if the question is marked multi-select.
- Prefer "finance" style metrics, no comparison, and concise granularity when unclear.
- Never invent option IDs.

Return ONLY valid JSON:
{"answers":{"question_id":"option_id"}}`

func (r *OpenAIHITLResponder) Answer(ctx context.Context, in HITLAnswerInput) (*HITLAnswerResult, error) {
	if len(in.Questions) == 0 {
		return nil, errors.New("no HITL questions to answer")
	}

	userPrompt := buildHITLUserPrompt(in)
	resp, content, err := r.requestAnswers(ctx, userPrompt)
	if err != nil {
		return nil, err
	}

	answers, err := parseHITLAnswers([]byte(content), in.Questions)
	if err != nil {
		return nil, fmt.Errorf("hitl responder parse failed: %w", err)
	}

	usage := buildJudgeUsage(r.model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	return &HITLAnswerResult{
		Answers: answers,
		Usage:   usage,
	}, nil
}

func (r *OpenAIHITLResponder) requestAnswers(ctx context.Context, userPrompt string) (*openai.ChatCompletion, string, error) {
	const maxTokens int64 = 512

	params := openai.ChatCompletionNewParams{
		Model: r.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(hitlSystemPrompt),
			openai.UserMessage(userPrompt),
		},
		MaxCompletionTokens: openai.Int(maxTokens),
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &openai.ResponseFormatJSONObjectParam{
				Type: oaiconstant.ValueOf[oaiconstant.JSONObject](),
			},
		},
	}

	lastErr := errors.New("hitl responder returned empty content")
	for range 2 {
		resp, err := r.client.Chat.Completions.New(ctx, params)
		if err != nil {
			lastErr = fmt.Errorf("hitl responder request failed: %w", err)
			continue
		}
		if len(resp.Choices) == 0 {
			lastErr = errors.New("hitl responder returned no choices")
			continue
		}
		msg := resp.Choices[0].Message
		content := extractChatMessageContent(msg)
		if content != "" {
			return resp, content, nil
		}
		if refusal := extractChatMessageRefusal(msg); refusal != "" {
			return nil, "", fmt.Errorf("hitl responder model refused: %s", refusal)
		}
		lastErr = errors.New("hitl responder returned empty content")
	}
	return nil, "", lastErr
}

func buildHITLUserPrompt(in HITLAnswerInput) string {
	var b strings.Builder
	if strings.TrimSpace(in.CaseID) != "" {
		b.WriteString("Case ID: ")
		b.WriteString(in.CaseID)
		b.WriteString("\n")
	}
	if strings.TrimSpace(in.CaseDescription) != "" {
		b.WriteString("Case description: ")
		b.WriteString(in.CaseDescription)
		b.WriteString("\n")
	}
	if strings.TrimSpace(in.UserPrompt) != "" {
		b.WriteString("User prompt:\n")
		b.WriteString(in.UserPrompt)
		b.WriteString("\n\n")
	}
	if len(in.OracleFacts) > 0 {
		b.WriteString("Oracle facts that the final answer should match:\n")
		for _, fact := range in.OracleFacts {
			if strings.TrimSpace(fact.Key) == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(fact.Key)
			b.WriteString(" => ")
			b.WriteString(fact.ExpectedValue)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("Questions:\n")
	for _, q := range in.Questions {
		b.WriteString("- ")
		b.WriteString(q.ID)
		b.WriteString(": ")
		b.WriteString(q.Text)
		b.WriteString(" [type=")
		b.WriteString(q.Type)
		b.WriteString("]\n")
		for _, opt := range q.Options {
			b.WriteString("  - ")
			b.WriteString(opt.ID)
			b.WriteString(": ")
			b.WriteString(opt.Label)
			if strings.TrimSpace(opt.Description) != "" {
				b.WriteString(" (")
				b.WriteString(opt.Description)
				b.WriteString(")")
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\nReturn JSON with one option ID per question.\n")
	return b.String()
}

func parseHITLAnswers(data []byte, questions []HITLQuestion) (map[string]string, error) {
	type payload struct {
		Answers map[string]string `json:"answers"`
	}

	decode := func(raw []byte) (payload, error) {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		var p payload
		if err := dec.Decode(&p); err != nil {
			return payload{}, err
		}
		return p, nil
	}

	p, err := decode(data)
	if err != nil {
		extracted, ok := extractJSONObject(data)
		if !ok {
			return nil, err
		}
		p, err = decode(extracted)
		if err != nil {
			return nil, err
		}
	}

	allowed := make(map[string]map[string]struct{}, len(questions))
	fallback := make(map[string]string, len(questions))
	for _, q := range questions {
		qid := strings.TrimSpace(q.ID)
		if qid == "" {
			continue
		}
		optionSet := make(map[string]struct{}, len(q.Options))
		for _, opt := range q.Options {
			id := strings.TrimSpace(opt.ID)
			if id == "" {
				continue
			}
			if fallback[qid] == "" {
				fallback[qid] = id
			}
			optionSet[id] = struct{}{}
		}
		allowed[qid] = optionSet
	}

	if len(allowed) == 0 {
		return nil, errors.New("no valid HITL question IDs")
	}

	result := make(map[string]string, len(allowed))
	for qid, options := range allowed {
		answer := strings.TrimSpace(p.Answers[qid])
		if answer == "" {
			fallbackAnswer := fallback[qid]
			if fallbackAnswer == "" {
				return nil, fmt.Errorf("question %q has no options", qid)
			}
			result[qid] = fallbackAnswer
			continue
		}
		if _, ok := options[answer]; !ok {
			fallbackAnswer := fallback[qid]
			if fallbackAnswer == "" {
				return nil, fmt.Errorf("question %q has no options", qid)
			}
			result[qid] = fallbackAnswer
			continue
		}
		result[qid] = answer
	}

	return result, nil
}
