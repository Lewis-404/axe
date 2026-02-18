package llm

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type ContentBlock struct {
	Type    string    `json:"type"`
	Text    string    `json:"text,omitempty"`
	ID      string    `json:"id,omitempty"`
	Name    string    `json:"name,omitempty"`
	Input   any       `json:"input,omitempty"`
	ToolID  string    `json:"tool_use_id,omitempty"`
	Content string    `json:"content,omitempty"`
	IsError bool      `json:"is_error,omitempty"`
}

type Message struct {
	Role    Role           `json:"role"`
	Content []ContentBlock `json:"content"`
}

type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema"`
}

type Request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
	Tools     []ToolDef `json:"tools,omitempty"`
}

type Response struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Role       Role           `json:"role"`
	Content    []ContentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Usage      Usage          `json:"usage"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type ErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}
