package openai

// ChatRequest Chat请求
type ChatRequest struct {
	Model          string          `json:"model"`                    // 模型名称
	Messages       []ChatMessage   `json:"messages"`                 // 消息列表
	Temperature    float32         `json:"temperature,omitempty"`    // 温度参数 (0-2)
	MaxTokens      int             `json:"max_tokens,omitempty"`     // 最大Token数
	Stream         bool            `json:"stream,omitempty"`         // 是否流式响应
	TopP           float32         `json:"top_p,omitempty"`          // 核采样参数
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"` // 响应格式（JSON mode）
}

// ResponseFormat 响应格式配置
type ResponseFormat struct {
	Type string `json:"type"` // 类型: text 或 json_object
}

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string `json:"role"`    // 角色: system, user, assistant
	Content string `json:"content"` // 消息内容
}

// ChatResponse Chat响应
type ChatResponse struct {
	ID      string       `json:"id"`      // 响应ID
	Object  string       `json:"object"`  // 对象类型
	Created int64        `json:"created"` // 创建时间戳
	Model   string       `json:"model"`   // 使用的模型
	Choices []ChatChoice `json:"choices"` // 选择列表
	Usage   Usage        `json:"usage"`   // Token使用情况
}

// ChatChoice 选择项
type ChatChoice struct {
	Index        int         `json:"index"`         // 索引
	Message      ChatMessage `json:"message"`       // 消息内容
	FinishReason string      `json:"finish_reason"` // 结束原因: stop, length, content_filter, null
}

// Usage Token使用情况
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`     // 提示Token数
	CompletionTokens int `json:"completion_tokens"` // 完成Token数
	TotalTokens      int `json:"total_tokens"`      // 总Token数
}

// StreamChunk 流式响应块
type StreamChunk struct {
	ID      string              `json:"id"`      // 响应ID
	Object  string              `json:"object"`  // 对象类型
	Created int64               `json:"created"` // 创建时间戳
	Model   string              `json:"model"`   // 使用的模型
	Choices []StreamChoiceDelta `json:"choices"` // 选择增量列表
}

// StreamChoiceDelta 流式选择增量
type StreamChoiceDelta struct {
	Index        int              `json:"index"`                   // 索引
	Delta        ChatMessageDelta `json:"delta"`                   // 消息增量
	FinishReason string           `json:"finish_reason,omitempty"` // 结束原因
}

// ChatMessageDelta 消息增量
type ChatMessageDelta struct {
	Role    string `json:"role,omitempty"`    // 角色
	Content string `json:"content,omitempty"` // 内容增量
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error ErrorDetail `json:"error"` // 错误详情
}

// ErrorDetail 错误详情
type ErrorDetail struct {
	Message string `json:"message"`        // 错误消息
	Type    string `json:"type"`           // 错误类型
	Code    string `json:"code,omitempty"` // 错误代码
}
