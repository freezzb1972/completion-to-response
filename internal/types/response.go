package types

// ResponsesAPIRequest represents an OpenAI Responses API request.
type ResponsesAPIRequest struct {
	Model            string          `json:"model"`
	Input            interface{}     `json:"input"`
	Instructions     string          `json:"instructions,omitempty"`
	MaxOutputTokens  *int            `json:"max_output_tokens,omitempty"`
	Temperature      *float64        `json:"temperature,omitempty"`
	TopP             *float64        `json:"top_p,omitempty"`
	Tools            []ResponsesTool `json:"tools,omitempty"`
	ToolChoice       interface{}     `json:"tool_choice,omitempty"`
	Text             *TextConfig     `json:"text,omitempty"`
	Stream           bool            `json:"stream,omitempty"`
	Store            *bool           `json:"store,omitempty"`
	PreviousResponseID string        `json:"previous_response_id,omitempty"`
	Reasoning        *ReasoningConfig `json:"reasoning,omitempty"`
	User             string          `json:"user,omitempty"`
}

type TextConfig struct {
	Format *TextFormat `json:"format,omitempty"`
}

type TextFormat struct {
	Type       string            `json:"type"`
	JSONSchema *TextFormatSchema `json:"json_schema,omitempty"`
}

type TextFormatSchema struct {
	Name   string                 `json:"name"`
	Strict bool                   `json:"strict"`
	Schema map[string]interface{} `json:"schema"`
}

type ReasoningConfig struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type ResponsesTool struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Strict      *bool                  `json:"strict,omitempty"`
}

// ResponsesAPIResponse represents an OpenAI Responses API response.
type ResponsesAPIResponse struct {
	ID         string          `json:"id"`
	Object     string          `json:"object"`
	CreatedAt  int64           `json:"created_at"`
	Model      string          `json:"model"`
	Status     string          `json:"status"`
	Output     []OutputItem    `json:"output"`
	OutputText string          `json:"output_text,omitempty"`
	Usage      *ResponsesUsage `json:"usage,omitempty"`
}

type ResponsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// OutputItem represents an item in the output array.
type OutputItem struct {
	ID        string        `json:"id"`
	Type      string        `json:"type"`
	Status    string        `json:"status,omitempty"`
	Role      string        `json:"role,omitempty"`
	Content   []ContentItem `json:"content,omitempty"`
	CallID    string        `json:"call_id,omitempty"`
	Name      string        `json:"name,omitempty"`
	Arguments string        `json:"arguments,omitempty"`
	Output    string        `json:"output,omitempty"`
	Summary   []interface{} `json:"summary,omitempty"`
}

type ContentItem struct {
	Type        string        `json:"type"`
	Text        string        `json:"text,omitempty"`
	Annotations []interface{} `json:"annotations,omitempty"`
	Logprobs    []interface{} `json:"logprobs,omitempty"`
}
