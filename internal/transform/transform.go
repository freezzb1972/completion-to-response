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
		// Collect late system/developer messages separately to avoid
		// breaking the assistant(tool_calls) → tool message sequence
		var lateSystem []string
		for i := 0; i < len(v); i++ {
			msg, ok := v[i].(map[string]interface{})
			if !ok {
				continue
			}
			itemType, _ := msg["type"].(string)

			if itemType == "function_call" {
				// Group consecutive function_call items into one assistant message
				var toolCalls []types.ToolCall
				var reasoningContent string
				for j := i; j < len(v); j++ {
					fc, ok := v[j].(map[string]interface{})
					if !ok {
						break
					}
					if fcType, _ := fc["type"].(string); fcType != "function_call" {
						break
					}
					callID, _ := fc["call_id"].(string)
					name, _ := fc["name"].(string)
					args, _ := fc["arguments"].(string)
					if reasoningContent == "" {
						if summary, ok := fc["summary"].([]interface{}); ok && len(summary) > 0 {
							if rc, ok := summary[0].(string); ok {
								reasoningContent = rc
							}
						}
					}
					toolCalls = append(toolCalls, types.ToolCall{
						ID:   callID,
						Type: "function",
						Function: types.FunctionCall{
							Name:      name,
							Arguments: args,
						},
					})
					i = j
				}
				messages = append(messages, types.ChatCompletionMessage{
					Role:             "assistant",
					Content:          "",
					ReasoningContent: reasoningContent,
					ToolCalls:        toolCalls,
				})
				continue
			}

			switch itemType {

			case "function_call_output":
				// Convert tool output to tool message
				callID, _ := msg["call_id"].(string)
				output, _ := msg["output"].(string)
				messages = append(messages, types.ChatCompletionMessage{
					Role:       "tool",
					ToolCallID: callID,
					Content:    output,
				})

			default:
				// Regular message (type "message" or no type)
				role, _ := msg["role"].(string)
				if mapRole(role) == "system" {
					// Defer system messages to avoid breaking tool call sequences
					if s, ok := convertContent(msg["content"]).(string); ok && s != "" {
						lateSystem = append(lateSystem, s)
					}
					continue
				}
				role = mapRole(role)
				if role == "" {
					// Skip items with no recognizable role or type
					continue
				}
				content := convertContent(msg["content"])
				messages = append(messages, types.ChatCompletionMessage{
					Role:    role,
					Content: content,
				})
			}
		}

		// Merge late system messages into the first system message
		if len(lateSystem) > 0 {
			allSystem := strings.Join(lateSystem, "\n\n")
			if len(messages) > 0 && messages[0].Role == "system" {
				if messages[0].Content != "" {
					messages[0].Content = messages[0].Content.(string) + "\n\n" + allSystem
				} else {
					messages[0].Content = allSystem
				}
			} else {
				// Prepend system message
				messages = append([]types.ChatCompletionMessage{{
					Role:    "system",
					Content: allSystem,
				}}, messages...)
			}
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

// toolCallState tracks accumulated tool call data across streaming chunks.
type toolCallState struct {
	ID     string
	Name   string
	argBuf strings.Builder
	started bool
}

// StreamConverter maintains state across streaming chunks to generate
// properly formatted Responses API SSE events.
type StreamConverter struct {
	respID          string
	msgID           string
	model           string
	created         int64
	output          []types.OutputItem
	textBuf         strings.Builder
	reasoningBuf    strings.Builder
	started         bool
	toolStates      map[int]*toolCallState
	toolUsed        bool
	nextOutputIndex int // output index for next function_call item (1+)
}

// NewStreamConverter creates a new stateful stream converter.
func NewStreamConverter(model string) *StreamConverter {
	return &StreamConverter{
		respID:          "resp_" + randomHex(16),
		msgID:           "msg_" + randomHex(16),
		model:           model,
		toolStates:      make(map[int]*toolCallState),
		nextOutputIndex: 1, // 0 = message, function calls start at 1
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

	// Reasoning content delta (thinking mode)
	if choice.Delta.ReasoningContent != "" {
		sc.reasoningBuf.WriteString(choice.Delta.ReasoningContent)
	}

	// Tool call deltas — accumulate across chunks
	for _, tc := range choice.Delta.ToolCalls {
		idx := tc.Index
		ts, ok := sc.toolStates[idx]
		if !ok {
			ts = &toolCallState{}
			sc.toolStates[idx] = ts
		}
		if tc.ID != "" {
			ts.ID = tc.ID
		}
		if tc.Function.Name != "" {
			ts.Name = tc.Function.Name
		}
		if tc.Function.Arguments != "" {
			if !ts.started {
				ts.started = true
				sc.toolUsed = true
				if ts.ID == "" {
					ts.ID = "fc_" + randomHex(16)
				}
				// emit response.output_item.added for function_call
				fcOutputIndex := sc.nextOutputIndex
				sc.nextOutputIndex++
				events = append(events, mustMarshal(map[string]interface{}{
					"type":         "response.output_item.added",
					"output_index": fcOutputIndex,
					"item": types.OutputItem{
						ID:     ts.ID,
						Type:   "function_call",
						Status: "in_progress",
						CallID: ts.ID,
						Name:   ts.Name,
					},
				}))
			}
			ts.argBuf.WriteString(tc.Function.Arguments)
			events = append(events, mustMarshal(map[string]interface{}{
				"type":         "response.function_call_arguments.delta",
				"output_index": sc.nextOutputIndex - 1,
				"item_id":      ts.ID,
				"delta":        tc.Function.Arguments,
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

		// response.output_item.done for message
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

		// Add function_call items from accumulated tool call state
		for idx := 0; idx < len(sc.toolStates); idx++ {
			ts, ok := sc.toolStates[idx]
			if !ok || !ts.started {
				continue
			}
		args := ts.argBuf.String()
			reasoning := sc.reasoningBuf.String()
			fcItem := types.OutputItem{
				ID:        ts.ID,
				Type:      "function_call",
				Status:    "completed",
				CallID:    ts.ID,
				Name:      ts.Name,
				Arguments: args,
			}
			if reasoning != "" {
				fcItem.Summary = []interface{}{reasoning}
			}
			output = append(output, fcItem)

			// response.function_call_arguments.done
			events = append(events, mustMarshal(map[string]interface{}{
				"type":         "response.function_call_arguments.done",
				"output_index": len(output) - 1,
				"item_id":      ts.ID,
				"arguments":    args,
			}))

			// response.output_item.done for function_call
			events = append(events, mustMarshal(map[string]interface{}{
				"type":         "response.output_item.done",
				"output_index": len(output) - 1,
				"item":         fcItem,
			}))
		}

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
