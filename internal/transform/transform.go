package transform

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/NoahStepheno/completion-to-response/internal/types"
)

// RequestToCompletion converts a Responses API request to a Chat Completions request.
func RequestToCompletion(req types.ResponsesAPIRequest) types.ChatCompletionRequest {
	// Build messages from input and instructions
	messages := buildMessages(req.Input, req.Instructions)

	// Convert tools from Responses format (internally tagged) to Chat Completions format (externally tagged)
	tools := convertTools(req.Tools)

	// Convert text.format to response_format if present
	var responseFormat *types.ResponseFormat
	if req.Text != nil && req.Text.Format != nil {
		if req.Text.Format.JSONSchema != nil {
			responseFormat = &types.ResponseFormat{
				Type: "json_object",
				JSONSchema: &types.ResponseFormatSchema{
					Name:   req.Text.Format.JSONSchema.Name,
					Strict: req.Text.Format.JSONSchema.Strict,
					Schema: req.Text.Format.JSONSchema.Schema,
				},
			}
		}
	}

	// Map max_output_tokens to max_tokens
	var maxTokens *int
	if req.MaxOutputTokens != nil && *req.MaxOutputTokens > 0 {
		maxTokens = req.MaxOutputTokens
	}

	return types.ChatCompletionRequest{
		Model:          req.Model,
		Messages:       messages,
		MaxTokens:      maxTokens,
		Temperature:    req.Temperature,
		TopP:           req.TopP,
		Stream:         req.Stream,
		Tools:          tools,
		ToolChoice:     req.ToolChoice,
		ResponseFormat: responseFormat,
		User:           req.User,
	}
}

// buildMessages converts input and instructions into Chat Completions messages.
func buildMessages(input interface{}, instructions string) []types.ChatCompletionMessage {
	var messages []types.ChatCompletionMessage

	// Add system message from instructions if present
	if instructions != "" {
		messages = append(messages, types.ChatCompletionMessage{
			Role:    "system",
			Content: instructions,
		})
	}

	// Convert input to messages
	switch v := input.(type) {
	case string:
		// Single string input becomes a user message
		messages = append(messages, types.ChatCompletionMessage{
			Role:    "user",
			Content: v,
		})
	case []interface{}:
		// Array of message objects (Responses API format)
		for _, item := range v {
			msg, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			role, _ := msg["role"].(string)
			// Map Responses API roles to Chat Completions roles
			role = mapRole(role)
			content := convertContent(msg["content"])

			messages = append(messages, types.ChatCompletionMessage{
				Role:    role,
				Content: content,
			})
		}
	}

	return messages
}

// mapRole maps Responses API roles to Chat Completions roles.
func mapRole(role string) string {
	switch role {
	case "developer":
		return "system"
	default:
		return role
	}
}

// convertContent converts Responses API content to Chat Completions content.
// Responses API: [{type: "input_text", text: "..."}]
// Chat Completions: plain string
func convertContent(content interface{}) interface{} {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		// Array of content parts - concatenate text values
		var parts []string
		for _, item := range v {
			part, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if text, ok := part["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		if len(parts) == 1 {
			return parts[0]
		}
		if len(parts) > 1 {
			return strings.Join(parts, "\n")
		}
	}
	return content
}

// convertTools converts tools from Responses format to Chat Completions format.
func convertTools(responsesTools []types.ResponsesTool) []types.Tool {
	if len(responsesTools) == 0 {
		return nil
	}

	tools := make([]types.Tool, 0, len(responsesTools))
	for _, rt := range responsesTools {
		if rt.Name == "" {
			continue
		}
		tools = append(tools, types.Tool{
			Type: "function",
			Function: &types.FunctionDef{
				Name:        rt.Name,
				Description: rt.Description,
				Parameters:  rt.Parameters,
				Strict:      rt.Strict,
			},
		})
	}
	return tools
}

// ResponseFromCompletion converts a Chat Completions response to a Responses API response.
func ResponseFromCompletion(resp types.ChatCompletionResponse) types.ResponsesAPIResponse {
	// Generate new response ID
	id := "resp_" + randomHex(16)

	// Get the first choice's message
	var output []types.OutputItem
	var outputText string

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		message := choice.Message

		// Convert content to output_text item
		if content, ok := message.Content.(string); ok && content != "" {
			outputText = content
			output = append(output, types.OutputItem{
				ID:     "msg_" + randomHex(16),
				Type:   "message",
				Status: "completed",
				Role:   "assistant",
				Content: []types.ContentItem{
					{
						Type:        "output_text",
						Text:        content,
						Annotations: []interface{}{},
					},
				},
			})
		}

		// Convert tool_calls to function_call items
		for _, tc := range message.ToolCalls {
			output = append(output, types.OutputItem{
				ID:       "fc_" + randomHex(16),
				Type:     "function_call",
				Status:   "completed",
				CallID:   tc.ID,
				Name:     tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
	}

	// Map finish_reason to status
	status := "completed"
	if len(resp.Choices) > 0 {
		finishReason := resp.Choices[0].FinishReason
		switch finishReason {
		case "tool_calls":
			status = "requires_action"
		case "length":
			status = "incomplete"
		}
	}

	// Convert usage
	var usage *types.ResponsesUsage
	if resp.Usage != nil {
		usage = &types.ResponsesUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		}
	}

	result := types.ResponsesAPIResponse{
		ID:         id,
		Object:     "response",
		CreatedAt:  resp.Created,
		Model:      resp.Model,
		Status:     status,
		Output:     output,
		Usage:      usage,
	}

	if outputText != "" {
		result.OutputText = outputText
	}

	return result
}

// StreamConverter maintains state across streaming chunks to generate
// properly formatted Responses API SSE events.
type StreamConverter struct {
	respID  string
	msgID   string
	model   string
	created int64
	output  []types.OutputItem
	textBuf strings.Builder
	started bool
}

// NewStreamConverter creates a new stateful stream converter.
func NewStreamConverter(model string) *StreamConverter {
	return &StreamConverter{
		respID: "resp_" + randomHex(16),
		msgID:  "msg_" + randomHex(16),
		model:  model,
	}
}

// OnChunk processes a streaming chunk and returns SSE events to forward.
// Each returned value is a complete JSON object to send as `data: {json}\n\n`.
func (sc *StreamConverter) OnChunk(chunk types.ChatCompletionStreamChunk) [][]byte {
	if len(chunk.Choices) == 0 {
		return nil
	}

	// Capture metadata from any chunk
	if chunk.ID != "" && sc.created == 0 {
		sc.created = chunk.Created
	}

	var events [][]byte

	choice := chunk.Choices[0]

	// First chunk with role: emit response.created + response.output_item.added
	if choice.Delta.Role != "" && !sc.started {
		sc.started = true

		resp := types.ResponsesAPIResponse{
			ID:        sc.respID,
			Object:    "response",
			CreatedAt: sc.created,
			Model:     sc.model,
			Status:    "in_progress",
			Output:    []types.OutputItem{},
		}
		events = append(events, mustMarshal(map[string]interface{}{
			"type":     "response.created",
			"response": resp,
		}))

		// response.in_progress
		events = append(events, mustMarshal(map[string]interface{}{
			"type":     "response.in_progress",
			"response": resp,
		}))

		// response.output_item.added
		item := types.OutputItem{
			ID:     sc.msgID,
			Type:   "message",
			Status: "in_progress",
			Role:   "assistant",
			Content: []types.ContentItem{
				{
					Type:        "output_text",
					Text:        "",
					Annotations: []interface{}{},
				},
			},
		}
		events = append(events, mustMarshal(map[string]interface{}{
			"type":         "response.output_item.added",
			"output_index": 0,
			"item":         item,
		}))

		// response.content_part.added
		events = append(events, mustMarshal(map[string]interface{}{
			"type":          "response.content_part.added",
			"output_index":  0,
			"content_index": 0,
			"part": types.ContentItem{
				Type:        "output_text",
				Text:        "",
				Annotations: []interface{}{},
			},
		}))
	}

	// Content delta
	if choice.Delta.Content != "" {
		sc.textBuf.WriteString(choice.Delta.Content)
		events = append(events, mustMarshal(map[string]interface{}{
			"type":          "response.output_text.delta",
			"output_index":  0,
			"content_index": 0,
			"delta":         choice.Delta.Content,
		}))
	}

	// Tool call deltas
	for _, tc := range choice.Delta.ToolCalls {
		if tc.Type == "function" && tc.Function.Arguments != "" {
			events = append(events, mustMarshal(map[string]interface{}{
				"type":          "response.function_call_arguments.delta",
				"output_index":  len(sc.output),
				"item_id":       tc.ID,
				"delta":         tc.Function.Arguments,
			}))
		}
	}

	// Finish reason
	if choice.FinishReason != nil {
		status := "completed"
		switch *choice.FinishReason {
		case "tool_calls":
			status = "requires_action"
		case "length":
			status = "incomplete"
		}

		fullText := sc.textBuf.String()

		// response.output_text.done
		events = append(events, mustMarshal(map[string]interface{}{
			"type":          "response.output_text.done",
			"output_index":  0,
			"content_index": 0,
			"text":          fullText,
		}))

		// response.content_part.done
		events = append(events, mustMarshal(map[string]interface{}{
			"type":          "response.content_part.done",
			"output_index":  0,
			"content_index": 0,
			"part": types.ContentItem{
				Type:        "output_text",
				Text:        fullText,
				Annotations: []interface{}{},
			},
		}))

		// response.output_item.done
		msgItem := types.OutputItem{
			ID:     sc.msgID,
			Type:   "message",
			Status: status,
			Role:   "assistant",
			Content: []types.ContentItem{
				{
					Type:        "output_text",
					Text:        fullText,
					Annotations: []interface{}{},
				},
			},
		}
		events = append(events, mustMarshal(map[string]interface{}{
			"type":         "response.output_item.done",
			"output_index": 0,
			"item":         msgItem,
		}))

		// Build final output for response.completed
		output := []types.OutputItem{msgItem}

		// response.completed with full response object
		finalResp := types.ResponsesAPIResponse{
			ID:         sc.respID,
			Object:     "response",
			CreatedAt:  sc.created,
			Model:      sc.model,
			Status:     status,
			Output:     output,
			Usage:      &types.ResponsesUsage{},
		}
		if fullText != "" {
			finalResp.OutputText = fullText
		}
		events = append(events, mustMarshal(map[string]interface{}{
			"type":     "response.completed",
			"response": finalResp,
		}))
	}

	return events
}

func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"type":"error","error":"marshal failed"}`)
	}
	return data
}

// randomHex generates a random hex string of the specified length (number of bytes * 2).
func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
