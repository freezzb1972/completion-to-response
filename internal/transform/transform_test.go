package transform

import (
	"encoding/json"
	"testing"

	"github.com/NoahStepheno/completion-to-response/internal/types"
)

func TestRequestToCompletion_StringInput(t *testing.T) {
	req := types.ResponsesAPIRequest{
		Model:    "gpt-4o",
		Input:    "Hello, world!",
		Stream:   false,
	}

	got := RequestToCompletion(req)

	if got.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", got.Model, "gpt-4o")
	}
	if len(got.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(got.Messages))
	}
	if got.Messages[0].Role != "user" {
		t.Errorf("Role = %q, want %q", got.Messages[0].Role, "user")
	}
	if got.Messages[0].Content != "Hello, world!" {
		t.Errorf("Content = %v, want %q", got.Messages[0].Content, "Hello, world!")
	}
}

func TestRequestToCompletion_WithInstructions(t *testing.T) {
	req := types.ResponsesAPIRequest{
		Model:        "gpt-4o",
		Input:        "Hi",
		Instructions: "You are a helpful assistant.",
	}

	got := RequestToCompletion(req)

	if len(got.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(got.Messages))
	}
	if got.Messages[0].Role != "system" {
		t.Errorf("First message Role = %q, want %q", got.Messages[0].Role, "system")
	}
	if got.Messages[1].Role != "user" {
		t.Errorf("Second message Role = %q, want %q", got.Messages[1].Role, "user")
	}
}

func TestRequestToCompletion_ArrayInput(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{"role": "system", "content": "Be helpful."},
		map[string]interface{}{"role": "user", "content": "Hello"},
	}
	req := types.ResponsesAPIRequest{
		Model: "gpt-4o",
		Input: input,
	}

	got := RequestToCompletion(req)

	if len(got.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(got.Messages))
	}
	if got.Messages[0].Role != "system" {
		t.Errorf("Messages[0].Role = %q, want %q", got.Messages[0].Role, "system")
	}
	if got.Messages[1].Role != "user" {
		t.Errorf("Messages[1].Role = %q, want %q", got.Messages[1].Role, "user")
	}
}

func TestRequestToCompletion_Tools(t *testing.T) {
	req := types.ResponsesAPIRequest{
		Model: "gpt-4o",
		Input: "What's the weather?",
		Tools: []types.ResponsesTool{
			{
				Type:        "function",
				Name:        "get_weather",
				Description: "Get weather info",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}

	got := RequestToCompletion(req)

	if len(got.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(got.Tools))
	}
	if got.Tools[0].Type != "function" {
		t.Errorf("Tool.Type = %q, want %q", got.Tools[0].Type, "function")
	}
	if got.Tools[0].Function == nil {
		t.Fatal("Tool.Function is nil")
	}
	if got.Tools[0].Function.Name != "get_weather" {
		t.Errorf("Function.Name = %q, want %q", got.Tools[0].Function.Name, "get_weather")
	}
}

func TestRequestToCompletion_TextFormat(t *testing.T) {
	req := types.ResponsesAPIRequest{
		Model: "gpt-4o",
		Input: "test",
		Text: &types.TextConfig{
			Format: &types.TextFormat{
				Type: "json_schema",
				JSONSchema: &types.TextFormatSchema{
					Name:   "test",
					Strict: true,
					Schema: map[string]interface{}{"type": "object"},
				},
			},
		},
	}

	got := RequestToCompletion(req)

	if got.ResponseFormat == nil {
		t.Fatal("ResponseFormat is nil")
	}
	if got.ResponseFormat.Type != "json_object" {
		t.Errorf("ResponseFormat.Type = %q, want %q", got.ResponseFormat.Type, "json_object")
	}
}

func TestRequestToCompletion_MaxOutputTokens(t *testing.T) {
	maxTokens := 100
	req := types.ResponsesAPIRequest{
		Model:           "gpt-4o",
		Input:           "test",
		MaxOutputTokens: &maxTokens,
	}

	got := RequestToCompletion(req)

	if got.MaxTokens == nil || *got.MaxTokens != 100 {
		t.Errorf("MaxTokens = %v, want 100", got.MaxTokens)
	}
}

func TestResponseFromCompletion_BasicMessage(t *testing.T) {
	compResp := types.ChatCompletionResponse{
		ID:      "chatcmpl-abc123",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "gpt-4o",
		Choices: []types.ChatCompletionChoice{
			{
				Index: 0,
				Message: types.ChatCompletionMessage{
					Role:    "assistant",
					Content: "Hello! How can I help you?",
				},
				FinishReason: "stop",
			},
		},
		Usage: &types.Usage{
			PromptTokens:     10,
			CompletionTokens: 8,
			TotalTokens:      18,
		},
	}

	got := ResponseFromCompletion(compResp)

	if got.Object != "response" {
		t.Errorf("Object = %q, want %q", got.Object, "response")
	}
	if got.ID == "" || got.ID[:5] != "resp_" {
		t.Errorf("ID = %q, want to start with 'resp_'", got.ID)
	}
	if got.CreatedAt != 1234567890 {
		t.Errorf("CreatedAt = %d, want 1234567890", got.CreatedAt)
	}
	if got.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", got.Model, "gpt-4o")
	}
	if got.Status != "completed" {
		t.Errorf("Status = %q, want %q", got.Status, "completed")
	}
	if got.OutputText != "Hello! How can I help you?" {
		t.Errorf("OutputText = %q, want %q", got.OutputText, "Hello! How can I help you?")
	}
	if len(got.Output) != 1 {
		t.Fatalf("len(Output) = %d, want 1", len(got.Output))
	}
	if got.Output[0].Type != "message" {
		t.Errorf("Output[0].Type = %q, want %q", got.Output[0].Type, "message")
	}
	if got.Output[0].Role != "assistant" {
		t.Errorf("Output[0].Role = %q, want %q", got.Output[0].Role, "assistant")
	}
	if got.Output[0].Status != "completed" {
		t.Errorf("Output[0].Status = %q, want %q", got.Output[0].Status, "completed")
	}
	if len(got.Output[0].Content) != 1 {
		t.Fatalf("len(Content) = %d, want 1", len(got.Output[0].Content))
	}
	if got.Output[0].Content[0].Type != "output_text" {
		t.Errorf("Content[0].Type = %q, want %q", got.Output[0].Content[0].Type, "output_text")
	}
	if got.Output[0].Content[0].Text != "Hello! How can I help you?" {
		t.Errorf("Content[0].Text = %q, want %q", got.Output[0].Content[0].Text, "Hello! How can I help you?")
	}
	if got.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if got.Usage.InputTokens != 10 {
		t.Errorf("Usage.InputTokens = %d, want 10", got.Usage.InputTokens)
	}
	if got.Usage.OutputTokens != 8 {
		t.Errorf("Usage.OutputTokens = %d, want 8", got.Usage.OutputTokens)
	}
}

func TestResponseFromCompletion_ToolCalls(t *testing.T) {
	compResp := types.ChatCompletionResponse{
		ID:      "chatcmpl-abc123",
		Created: 1234567890,
		Model:   "gpt-4o",
		Choices: []types.ChatCompletionChoice{
			{
				Index: 0,
				Message: types.ChatCompletionMessage{
					Role:    "assistant",
					Content: "",
					ToolCalls: []types.ToolCall{
						{
							ID:   "call_abc123",
							Type: "function",
							Function: types.FunctionCall{
								Name:      "get_weather",
								Arguments: `{"location":"Paris"}`,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
	}

	got := ResponseFromCompletion(compResp)

	if got.Status != "requires_action" {
		t.Errorf("Status = %q, want %q", got.Status, "requires_action")
	}
	if len(got.Output) != 1 {
		t.Fatalf("len(Output) = %d, want 1", len(got.Output))
	}
	toolItem := got.Output[0]
	if toolItem.Type != "function_call" {
		t.Errorf("Type = %q, want %q", toolItem.Type, "function_call")
	}
	if toolItem.CallID != "call_abc123" {
		t.Errorf("CallID = %q, want %q", toolItem.CallID, "call_abc123")
	}
	if toolItem.Name != "get_weather" {
		t.Errorf("Name = %q, want %q", toolItem.Name, "get_weather")
	}
	if toolItem.Arguments != `{"location":"Paris"}` {
		t.Errorf("Arguments = %q, want %q", toolItem.Arguments, `{"location":"Paris"}`)
	}
}

func TestResponseFromCompletion_FinishReasonLength(t *testing.T) {
	compResp := types.ChatCompletionResponse{
		ID:      "chatcmpl-abc",
		Created: 123,
		Model:   "gpt-4o",
		Choices: []types.ChatCompletionChoice{
			{
				Index: 0,
				Message: types.ChatCompletionMessage{
					Role:    "assistant",
					Content: "truncated...",
				},
				FinishReason: "length",
			},
		},
	}

	got := ResponseFromCompletion(compResp)

	if got.Status != "incomplete" {
		t.Errorf("Status = %q, want %q", got.Status, "incomplete")
	}
}

func TestResponseFromCompletion_IDFormat(t *testing.T) {
	compResp := types.ChatCompletionResponse{
		ID:      "chatcmpl-abc",
		Created: 123,
		Model:   "gpt-4o",
		Choices: []types.ChatCompletionChoice{
			{
				Index: 0,
				Message: types.ChatCompletionMessage{
					Role:    "assistant",
					Content: "test",
				},
				FinishReason: "stop",
			},
		},
	}

	got := ResponseFromCompletion(compResp)

	if len(got.ID) != 5+32 { // "resp_" + 32 hex chars
		t.Errorf("len(ID) = %d, want %d", len(got.ID), 5+32)
	}
	msgID := got.Output[0].ID
	if len(msgID) != 4+32 { // "msg_" + 32 hex chars
		t.Errorf("len(msgID) = %d, want %d", len(msgID), 4+32)
	}
}

func TestResponseFromCompletion_EmptyChoices(t *testing.T) {
	compResp := types.ChatCompletionResponse{
		ID:      "chatcmpl-abc",
		Created: 123,
		Model:   "gpt-4o",
		Choices: []types.ChatCompletionChoice{},
	}

	got := ResponseFromCompletion(compResp)

	if got.Object != "response" {
		t.Errorf("Object = %q, want %q", got.Object, "response")
	}
	if len(got.Output) != 0 {
		t.Errorf("len(Output) = %d, want 0", len(got.Output))
	}
	if got.Status != "completed" {
		t.Errorf("Status = %q, want %q", got.Status, "completed")
	}
}

func TestStreamConverter_FullFlow(t *testing.T) {
	sc := NewStreamConverter("gpt-4o")

	// First chunk: role
	events := sc.OnChunk(types.ChatCompletionStreamChunk{
		ID:    "chatcmpl-abc",
		Model: "gpt-4o",
		Choices: []types.ChatCompletionStreamChoice{
			{Index: 0, Delta: types.ChatCompletionStreamDelta{Role: "assistant"}},
		},
	})
	if len(events) < 3 {
		t.Fatalf("role chunk: got %d events, want >= 3", len(events))
	}
	var first map[string]interface{}
	json.Unmarshal(events[0], &first)
	if first["type"] != "response.created" {
		t.Errorf("first event type = %v, want response.created", first["type"])
	}

	// Content delta
	events = sc.OnChunk(types.ChatCompletionStreamChunk{
		Choices: []types.ChatCompletionStreamChoice{
			{Index: 0, Delta: types.ChatCompletionStreamDelta{Content: "Hello "}},
		},
	})
	if len(events) != 1 {
		t.Fatalf("content chunk: got %d events, want 1", len(events))
	}
	var delta map[string]interface{}
	json.Unmarshal(events[0], &delta)
	if delta["type"] != "response.output_text.delta" {
		t.Errorf("delta type = %v, want response.output_text.delta", delta["type"])
	}
	if delta["delta"] != "Hello " {
		t.Errorf("delta = %v, want 'Hello '", delta["delta"])
	}

	// More content
	sc.OnChunk(types.ChatCompletionStreamChunk{
		Choices: []types.ChatCompletionStreamChoice{
			{Index: 0, Delta: types.ChatCompletionStreamDelta{Content: "world"}},
		},
	})

	// Finish
	reason := "stop"
	events = sc.OnChunk(types.ChatCompletionStreamChunk{
		Choices: []types.ChatCompletionStreamChoice{
			{Index: 0, Delta: types.ChatCompletionStreamDelta{}, FinishReason: &reason},
		},
	})
	// Should have: output_text.done, content_part.done, output_item.done, response.completed
	if len(events) < 4 {
		t.Fatalf("finish chunk: got %d events, want >= 4", len(events))
	}

	var last map[string]interface{}
	json.Unmarshal(events[len(events)-1], &last)
	if last["type"] != "response.completed" {
		t.Errorf("last event type = %v, want response.completed", last["type"])
	}
	resp := last["response"].(map[string]interface{})
	if resp["status"] != "completed" {
		t.Errorf("response status = %v, want completed", resp["status"])
	}
}

func TestStreamConverter_EmptyChoices(t *testing.T) {
	sc := NewStreamConverter("gpt-4o")
	events := sc.OnChunk(types.ChatCompletionStreamChunk{
		Choices: []types.ChatCompletionStreamChoice{},
	})
	if events != nil {
		t.Errorf("events = %v, want nil", events)
	}
}

func TestStreamConverter_RequiresAction(t *testing.T) {
	sc := NewStreamConverter("gpt-4o")
	// Start stream
	sc.OnChunk(types.ChatCompletionStreamChunk{
		Choices: []types.ChatCompletionStreamChoice{
			{Index: 0, Delta: types.ChatCompletionStreamDelta{Role: "assistant"}},
		},
	})
	// Finish with tool_calls
	reason := "tool_calls"
	events := sc.OnChunk(types.ChatCompletionStreamChunk{
		Choices: []types.ChatCompletionStreamChoice{
			{Index: 0, Delta: types.ChatCompletionStreamDelta{}, FinishReason: &reason},
		},
	})
	var last map[string]interface{}
	json.Unmarshal(events[len(events)-1], &last)
	resp := last["response"].(map[string]interface{})
	if resp["status"] != "requires_action" {
		t.Errorf("status = %v, want requires_action", resp["status"])
	}
}

func TestResponseJSON(t *testing.T) {
	compResp := types.ChatCompletionResponse{
		ID:      "chatcmpl-abc",
		Created: 1234567890,
		Model:   "gpt-4o",
		Choices: []types.ChatCompletionChoice{
			{
				Index: 0,
				Message: types.ChatCompletionMessage{
					Role:    "assistant",
					Content: "Hello!",
				},
				FinishReason: "stop",
			},
		},
	}

	got := ResponseFromCompletion(compResp)
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if parsed["object"] != "response" {
		t.Errorf("object = %v, want 'response'", parsed["object"])
	}
	if parsed["status"] != "completed" {
		t.Errorf("status = %v, want 'completed'", parsed["status"])
	}
	output, ok := parsed["output"].([]interface{})
	if !ok || len(output) != 1 {
		t.Fatalf("output = %v, want array of 1", parsed["output"])
	}
	item := output[0].(map[string]interface{})
	if item["type"] != "message" {
		t.Errorf("item type = %v, want 'message'", item["type"])
	}
}
