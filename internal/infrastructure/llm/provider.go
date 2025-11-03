package llm

import (
	"context"
)

// Provider LLM提供商接口
// 定义了所有LLM Provider必须实现的核心功能
type Provider interface {
	// Name 返回Provider名称 (openai, anthropic, ollama等)
	Name() string

	// Generate 生成文本响应
	// 参数:
	//   - ctx: 上下文，用于控制超时和取消
	//   - prompt: 用户提示词
	//   - opts: 可选配置参数
	// 返回:
	//   - string: 生成的文本内容
	//   - error: 错误信息
	Generate(ctx context.Context, prompt string, opts ...Option) (string, error)

	// GenerateStream 流式生成文本
	// 参数:
	//   - ctx: 上下文
	//   - prompt: 用户提示词
	//   - opts: 可选配置参数
	// 返回:
	//   - <-chan string: 文本流通道，每个token或文本片段会通过此通道发送
	//   - <-chan error: 错误通道，如果发生错误会通过此通道发送
	GenerateStream(ctx context.Context, prompt string, opts ...Option) (<-chan string, <-chan error)

	// GenerateStructured 生成结构化输出（JSON）
	// 使用JSON mode或Function Calling实现结构化输出
	// 参数:
	//   - ctx: 上下文
	//   - prompt: 用户提示词
	//   - schema: 期望的输出结构，将被解析为JSON
	//   - opts: 可选配置参数
	// 返回:
	//   - error: 错误信息，成功时schema会被填充数据
	GenerateStructured(ctx context.Context, prompt string, schema interface{}, opts ...Option) error

	// IsAvailable 检查Provider是否可用
	// 用于健康检查和故障转移
	// 返回:
	//   - bool: true表示可用，false表示不可用
	IsAvailable() bool
}

// Option 生成选项函数类型
// 使用函数式选项模式来配置生成参数
type Option func(*GenerateOptions)

// GenerateOptions 生成配置
// 包含所有可配置的生成参数
type GenerateOptions struct {
	Model        string    // 模型名称，如gpt-4, gpt-3.5-turbo等
	Temperature  float32   // 生成温度 0.0-2.0，越高越随机
	MaxTokens    int       // 最大生成token数
	SystemPrompt string    // 系统提示词，定义AI的行为和角色
	Messages     []Message // 多轮对话历史
	JSONMode     bool      // 是否启用JSON模式（强制输出JSON）
}

// Message 对话消息
// 表示单条对话消息
type Message struct {
	Role    string // 角色: system, user, assistant
	Content string // 消息内容
}

// WithModel 设置模型
func WithModel(model string) Option {
	return func(opts *GenerateOptions) {
		opts.Model = model
	}
}

// WithTemperature 设置温度
// temperature越高，输出越随机；越低，输出越确定
func WithTemperature(t float32) Option {
	return func(opts *GenerateOptions) {
		opts.Temperature = t
	}
}

// WithMaxTokens 设置最大token数
func WithMaxTokens(n int) Option {
	return func(opts *GenerateOptions) {
		opts.MaxTokens = n
	}
}

// WithSystemPrompt 设置系统提示词
// 系统提示词用于定义AI的行为、角色和输出格式
func WithSystemPrompt(prompt string) Option {
	return func(opts *GenerateOptions) {
		opts.SystemPrompt = prompt
	}
}

// WithMessages 设置多轮对话历史
// 用于实现上下文对话
func WithMessages(messages []Message) Option {
	return func(opts *GenerateOptions) {
		opts.Messages = messages
	}
}

// WithJSONMode 启用JSON模式
// 强制模型输出有效的JSON格式
func WithJSONMode(enabled bool) Option {
	return func(opts *GenerateOptions) {
		opts.JSONMode = enabled
	}
}

// ApplyOptions 应用选项到默认配置
// 内部辅助函数，用于合并默认值和用户选项
func ApplyOptions(defaults *GenerateOptions, opts ...Option) *GenerateOptions {
	options := &GenerateOptions{
		Model:       defaults.Model,
		Temperature: defaults.Temperature,
		MaxTokens:   defaults.MaxTokens,
	}

	for _, opt := range opts {
		opt(options)
	}

	return options
}
