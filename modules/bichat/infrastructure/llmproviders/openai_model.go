package llmproviders

import (
	"context"
	"errors"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sashabaranov/go-openai"
)

// OpenAIModel implements the agents.Model interface using OpenAI's API.
// It provides both blocking and streaming completion modes with tool calling support.
//
// CITATION EXTRACTION:
// This implementation includes PARTIAL support for extracting citations from LLM responses.
// Currently, only citation markers ([1], [2], etc.) are extracted from response content.
// Full citation metadata (URLs, titles, excerpts) requires OpenAI API support.
//
// Current State (as of 2025-02):
//   - Citation markers in text: IMPLEMENTED (regex-based extraction)
//   - Citation metadata (URLs, titles): NOT AVAILABLE (OpenAI API limitation)
//   - go-openai SDK version: v1.40.1 (no citation metadata fields)
//
// Future Improvements:
// When OpenAI adds citation metadata to their API (Responses API or ChatCompletion extensions):
//  1. Update go-openai SDK to latest version
//  2. Check if ChatCompletionResponse includes metadata/annotations fields
//  3. Modify extractCitationsFromResponse() to parse metadata
//  4. Map metadata to types.Citation with full URL/title/excerpt
//
// Alternative: Implement via web_search tool results
//   - When WebSearchTool.Call() is implemented (see pkg/bichat/tools/web_search.go)
//   - Parse tool result JSON to extract search results
//   - Map tool results to citations with full metadata
//   - This is the RECOMMENDED approach until native API support is available
type OpenAIModel struct {
	client    *openai.Client
	modelName string
}

// NewOpenAIModel creates a new OpenAI model from environment variables.
// It reads OPENAI_API_KEY (required) and OPENAI_MODEL (optional, defaults to gpt-4).
// Returns an error if OPENAI_API_KEY is not set.
func NewOpenAIModel() (agents.Model, error) {
	const op serrors.Op = "llmproviders.NewOpenAIModel"

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, serrors.E(op, "OPENAI_API_KEY environment variable is required")
	}

	modelName := os.Getenv("OPENAI_MODEL")
	if modelName == "" {
		modelName = openai.GPT4 // Default to gpt-4
	}

	client := openai.NewClient(apiKey)

	return &OpenAIModel{
		client:    client,
		modelName: modelName,
	}, nil
}

// Generate sends a blocking completion request to OpenAI.
// It supports tool calling, JSON mode, and temperature control via options.
func (m *OpenAIModel) Generate(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
	const op serrors.Op = "OpenAIModel.Generate"

	// Apply options
	config := agents.ApplyGenerateOptions(opts...)

	// Build OpenAI request
	oaiReq := m.buildChatCompletionRequest(req, config)

	// Call OpenAI API
	resp, err := m.client.CreateChatCompletion(ctx, oaiReq)
	if err != nil {
		return nil, serrors.E(op, err, "OpenAI API request failed")
	}

	// Validate response
	if len(resp.Choices) == 0 {
		return nil, serrors.E(op, "OpenAI returned empty response")
	}

	choice := resp.Choices[0]

	// Build message with content and tool calls
	msg := types.Message{
		Role:    types.RoleAssistant,
		Content: choice.Message.Content,
	}

	// Convert tool calls
	if len(choice.Message.ToolCalls) > 0 {
		msg.ToolCalls = make([]types.ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			msg.ToolCalls[i] = types.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}
	}

	// Extract citations from response metadata (if present)
	// TODO: OpenAI Responses API returns citations in metadata when web_search tool is used
	// Once go-openai library supports citation metadata, extract and map citations here
	// Expected format: response metadata contains web search results with title, URL, snippet
	citations := extractCitationsFromResponse(&resp)
	if len(citations) > 0 {
		msg.Citations = citations
	}

	// Build token usage with cache token support
	usage := types.TokenUsage{
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
	}

	// Extract cache tokens from PromptTokenDetails if present
	// OpenAI's prompt caching: cached tokens are served from cache (read)
	if resp.Usage.PromptTokensDetails != nil {
		if resp.Usage.PromptTokensDetails.CachedTokens > 0 {
			usage.CacheReadTokens = resp.Usage.PromptTokensDetails.CachedTokens
		}
	}

	return &agents.Response{
		Message:      msg,
		Usage:        usage,
		FinishReason: string(choice.FinishReason),
	}, nil
}

// Stream sends a streaming completion request to OpenAI.
// Returns a Generator that yields Chunk objects as tokens arrive.
func (m *OpenAIModel) Stream(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (types.Generator[agents.Chunk], error) {
	const op serrors.Op = "OpenAIModel.Stream"

	config := agents.ApplyGenerateOptions(opts...)

	// Build OpenAI streaming request
	oaiReq := m.buildChatCompletionRequest(req, config)

	// Create stream immediately to catch errors early
	stream, err := m.client.CreateChatCompletionStream(ctx, oaiReq)
	if err != nil {
		return nil, serrors.E(op, err, "failed to create OpenAI stream")
	}

	return types.NewGenerator(ctx, func(genCtx context.Context, yield func(agents.Chunk) bool) error {
		defer stream.Close()

		// Accumulate tool calls across chunks
		toolCallsMap := make(map[int]*types.ToolCall)
		var totalUsage *types.TokenUsage

		// Stream chunks
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				// Send final chunk with usage if available
				if totalUsage != nil {
					if !yield(agents.Chunk{
						Done:  true,
						Usage: totalUsage,
					}) {
						return nil // Generator cancelled
					}
				}
				return nil // Stream complete
			}
			if err != nil {
				return serrors.E(op, err, "stream receive failed")
			}

			if len(response.Choices) == 0 {
				continue
			}

			choice := response.Choices[0]
			chunk := agents.Chunk{
				Delta:        choice.Delta.Content,
				FinishReason: string(choice.FinishReason),
			}

			// Handle tool call deltas
			if len(choice.Delta.ToolCalls) > 0 {
				for _, tcDelta := range choice.Delta.ToolCalls {
					// Index can be nil in some cases, use 0 as default
					index := 0
					if tcDelta.Index != nil {
						index = *tcDelta.Index
					}

					if toolCallsMap[index] == nil {
						toolCallsMap[index] = &types.ToolCall{
							ID:   tcDelta.ID,
							Name: tcDelta.Function.Name,
						}
					}
					// Accumulate arguments
					toolCallsMap[index].Arguments += tcDelta.Function.Arguments
				}

				// Convert map to slice for chunk
				// Sort indices to ensure deterministic order
				indices := make([]int, 0, len(toolCallsMap))
				for idx := range toolCallsMap {
					indices = append(indices, idx)
				}
				sort.Ints(indices)

				chunk.ToolCalls = make([]types.ToolCall, 0, len(toolCallsMap))
				for _, idx := range indices {
					if tc := toolCallsMap[idx]; tc != nil {
						chunk.ToolCalls = append(chunk.ToolCalls, *tc)
					}
				}
			}

			// Check for usage (sent in final chunk by OpenAI)
			if response.Usage != nil && response.Usage.TotalTokens > 0 {
				totalUsage = &types.TokenUsage{
					PromptTokens:     response.Usage.PromptTokens,
					CompletionTokens: response.Usage.CompletionTokens,
					TotalTokens:      response.Usage.TotalTokens,
				}

				// Extract cache tokens from PromptTokenDetails if present
				if response.Usage.PromptTokensDetails != nil {
					if response.Usage.PromptTokensDetails.CachedTokens > 0 {
						totalUsage.CacheReadTokens = response.Usage.PromptTokensDetails.CachedTokens
					}
				}

				chunk.Usage = totalUsage
			}

			// Mark as done if finish reason present
			if chunk.FinishReason != "" {
				chunk.Done = true
			}

			// Yield chunk
			if !yield(chunk) {
				return nil // Generator cancelled
			}

			if chunk.Done {
				return nil // Stream complete
			}
		}
	}, types.WithBufferSize(10)), nil // Buffer up to 10 chunks
}

// Info returns model metadata including capabilities.
func (m *OpenAIModel) Info() agents.ModelInfo {
	return agents.ModelInfo{
		Name:     m.modelName,
		Provider: "openai",
		Capabilities: []agents.Capability{
			agents.CapabilityStreaming,
			agents.CapabilityTools,
			agents.CapabilityJSONMode,
		},
	}
}

// HasCapability checks if this model supports a specific capability.
func (m *OpenAIModel) HasCapability(capability agents.Capability) bool {
	return m.Info().HasCapability(capability)
}

// Pricing returns pricing information for this OpenAI model.
// Prices are in USD per 1 million tokens.
func (m *OpenAIModel) Pricing() agents.ModelPricing {
	// Default pricing for common OpenAI models
	// Source: https://openai.com/api/pricing/ (as of 2025-02)
	pricing := map[string]agents.ModelPricing{
		openai.GPT4o: {
			Currency:        "USD",
			InputPer1M:      2.50,
			OutputPer1M:     10.00,
			CacheWritePer1M: 1.25,  // 50% of input price
			CacheReadPer1M:  0.625, // 25% of input price
		},
		openai.GPT4oMini: {
			Currency:        "USD",
			InputPer1M:      0.150,
			OutputPer1M:     0.600,
			CacheWritePer1M: 0.075, // 50% of input price
			CacheReadPer1M:  0.038, // 25% of input price
		},
		openai.GPT4Turbo: {
			Currency:        "USD",
			InputPer1M:      10.00,
			OutputPer1M:     30.00,
			CacheWritePer1M: 0, // No prompt caching for older models
			CacheReadPer1M:  0,
		},
		openai.GPT4: {
			Currency:        "USD",
			InputPer1M:      30.00,
			OutputPer1M:     60.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0,
		},
		openai.GPT3Dot5Turbo: {
			Currency:        "USD",
			InputPer1M:      0.50,
			OutputPer1M:     1.50,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0,
		},
	}

	// Return specific pricing or default fallback
	if p, exists := pricing[m.modelName]; exists {
		return p
	}

	// Fallback: assume GPT-4o pricing for unknown models
	return agents.ModelPricing{
		Currency:        "USD",
		InputPer1M:      2.50,
		OutputPer1M:     10.00,
		CacheWritePer1M: 1.25,
		CacheReadPer1M:  0.625,
	}
}

// buildChatCompletionRequest converts agents.Request to OpenAI format.
func (m *OpenAIModel) buildChatCompletionRequest(req agents.Request, config agents.GenerateConfig) openai.ChatCompletionRequest {
	// Convert messages
	messages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openai.ChatCompletionMessage{
			Role: string(msg.Role),
		}

		// Handle vision: if message has attachments, use multipart content
		if len(msg.Attachments) > 0 {
			// Build multipart content with text + images
			contentParts := make([]openai.ChatMessagePart, 0, 1+len(msg.Attachments))

			// Add text content if present
			if msg.Content != "" {
				contentParts = append(contentParts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: msg.Content,
				})
			}

			// Add image URLs (low-detail mode = 85 tokens per image)
			for _, attachment := range msg.Attachments {
				contentParts = append(contentParts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL:    attachment.FilePath, // FilePath stores the URL
						Detail: openai.ImageURLDetailLow,
					},
				})
			}

			messages[i].MultiContent = contentParts
		} else {
			// Simple text content
			messages[i].Content = msg.Content
		}

		// Add tool call ID for tool messages
		if msg.ToolCallID != nil {
			messages[i].ToolCallID = *msg.ToolCallID
		}

		// Add tool calls for assistant messages
		if len(msg.ToolCalls) > 0 {
			messages[i].ToolCalls = make([]openai.ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				messages[i].ToolCalls[j] = openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				}
			}
		}
	}

	// Build base request
	oaiReq := openai.ChatCompletionRequest{
		Model:    m.modelName,
		Messages: messages,
	}

	// Apply max tokens
	if config.MaxTokens != nil {
		oaiReq.MaxTokens = *config.MaxTokens
	}

	// Apply temperature
	if config.Temperature != nil {
		oaiReq.Temperature = float32(*config.Temperature)
	}

	// Apply JSON mode
	if config.JSONMode && m.HasCapability(agents.CapabilityJSONMode) {
		oaiReq.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

	// Convert tools
	if len(req.Tools) > 0 {
		oaiReq.Tools = make([]openai.Tool, 0, len(req.Tools))
		for _, tool := range req.Tools {
			// Skip code_interpreter tool - it's handled via Assistants API
			// TODO: Implement code_interpreter via OpenAI Assistants API
			// For now, code_interpreter tool is a placeholder and won't be sent to OpenAI
			if tool.Name() == "code_interpreter" {
				continue
			}

			// Include web_search tool - OpenAI will call it like any other tool
			// The tool implementation (WebSearchTool.Call) needs to execute actual web search
			// TODO: Implement web search provider (Tavily, Serper, Bing, etc.) in WebSearchTool.Call
			// TODO: Once OpenAI Responses API with native web search is available, update this

			oaiReq.Tools = append(oaiReq.Tools, openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        tool.Name(),
					Description: tool.Description(),
					Parameters:  tool.Parameters(),
				},
			})
		}
	}

	return oaiReq
}

// extractCitationsFromResponse extracts web search citations from OpenAI response metadata.
//
// CURRENT LIMITATION: OpenAI's ChatCompletion API does NOT currently expose web search citations
// in response metadata. The go-openai SDK (v1.40.1) does not include citation fields in
// ChatCompletionResponse or ChatCompletionMessage structures.
//
// As of 2025-02, OpenAI's web search capability is:
// - NOT available via native API (Responses API with web search is not public)
// - Only accessible via tool calling (custom web_search tool implementation)
// - When implemented via tools, citations must be extracted from tool result content
//
// IMPLEMENTATION OPTIONS:
//
// Option 1: Parse citation markers from response content (FRAGILE)
//   - Scan for patterns like [1], [2], [3] in message content
//   - Extract citation numbers and their positions
//   - Problem: No metadata to map numbers to actual URLs/sources
//   - Only works if LLM formats citations consistently
//
// Option 2: Extract from web_search tool results (RECOMMENDED)
//   - When web_search tool is called, parse its JSON result
//   - Tool result contains: query, results[{title, url, snippet}]
//   - Map tool results to citations and attach to message
//   - Requires WebSearchTool.Call() to return structured data
//
// Option 3: Wait for OpenAI Responses API (FUTURE)
//   - OpenAI may add native web search with citation metadata
//   - Expected fields: response.metadata.web_search_results[]
//   - SDK would need update to expose these fields
//
// TODO: Implement Option 2 when WebSearchTool.Call() is implemented
// Expected tool result format:
//
//	{
//	  "query": "user's search query",
//	  "results": [
//	    {
//	      "index": 1,
//	      "title": "Page Title",
//	      "url": "https://example.com",
//	      "snippet": "Relevant excerpt from page..."
//	    }
//	  ]
//	}
//
// Then citations can be built from tool messages:
// 1. Find tool messages with name="web_search" in conversation history
// 2. Parse tool result JSON to extract results array
// 3. Map each result to types.Citation with proper indexes
// 4. Return citations array for message
//
// For now, return nil - citations will be populated when:
// - WebSearchTool is implemented with a real search provider (Tavily, Serper, Bing)
// - OR OpenAI adds native citation metadata to ChatCompletionResponse
func extractCitationsFromResponse(resp *openai.ChatCompletionResponse) []types.Citation {
	// Step 1: Check if response has metadata (future-proofing)
	// Note: ChatCompletionResponse does not have Metadata field as of go-openai v1.40.1
	// This code is here for when SDK is updated

	// Step 2: Attempt to parse citation markers from content (fallback, fragile)
	// This only extracts positions, not actual source data
	if len(resp.Choices) > 0 {
		content := resp.Choices[0].Message.Content
		if content != "" {
			// Extract citation markers like [1], [2], [3]
			citations := extractCitationMarkersFromContent(content)
			if len(citations) > 0 {
				return citations
			}
		}
	}

	// Step 3: No citations found
	return nil
}

// extractCitationMarkersFromContent parses citation markers like [1], [2] from text.
// This is a FALLBACK implementation that only extracts positions, not source metadata.
//
// LIMITATIONS:
// - Cannot map citation numbers to actual URLs/sources (no metadata available)
// - Assumes LLM formats citations as [N] where N is a number
// - Does not handle all citation formats (e.g., [1,2], [1-3], footnotes)
// - Fragile: LLM may change citation format between responses
//
// Returns Citation objects with:
// - Type: "web" (assumed)
// - Title: "Citation [N]" (placeholder, actual title unknown)
// - URL: "" (empty, no metadata available)
// - Excerpt: "" (empty, no metadata available)
// - StartIndex/EndIndex: Position of [N] in content
//
// This provides minimal citation tracking until proper metadata is available.
func extractCitationMarkersFromContent(content string) []types.Citation {
	// Pattern: [1], [2], [3], etc. (number in square brackets)
	// Matches: [1] [23] [456]
	// Does NOT match: [a] [1a] [,] [1,2]
	pattern := regexp.MustCompile(`\[(\d+)\]`)
	matches := pattern.FindAllStringSubmatchIndex(content, -1)

	if len(matches) == 0 {
		return nil
	}

	citations := make([]types.Citation, 0, len(matches))
	seen := make(map[int]bool) // Track unique citation numbers

	for _, match := range matches {
		// match[0], match[1]: full match positions (e.g., "[1]")
		// match[2], match[3]: first capture group positions (e.g., "1")
		fullStart := match[0]
		fullEnd := match[1]
		numStart := match[2]
		numEnd := match[3]

		// Extract citation number
		numStr := content[numStart:numEnd]
		num, err := strconv.Atoi(numStr)
		if err != nil {
			continue // Should never happen with \d+ pattern
		}

		// Skip duplicate citation numbers
		if seen[num] {
			continue
		}
		seen[num] = true

		// Create citation with position info but no metadata
		citation := types.Citation{
			Type:       "web", // Assumed type (most citations are web)
			Title:      "Citation [" + numStr + "]",
			URL:        "", // Unknown without metadata
			Excerpt:    "", // Unknown without metadata
			StartIndex: fullStart,
			EndIndex:   fullEnd,
		}

		citations = append(citations, citation)
	}

	return citations
}
