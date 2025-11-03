package contracts

import (
	"context"
)

// LLMService LLM应用服务接口
// 提供应用层的LLM调用能力，封装业务逻辑
type LLMService interface {
	// GenerateText 生成文本
	// 输入prompt和选项，返回生成的文本
	GenerateText(ctx context.Context, prompt string, opts ...LLMOption) (string, error)

	// GenerateTextStream 流式生成文本
	// 实时返回生成的文本片段，适用于长文本或需要实时反馈的场景
	GenerateTextStream(ctx context.Context, prompt string, opts ...LLMOption) (<-chan string, <-chan error)

	// GenerateStructured 生成结构化输出
	// 输入prompt和期望的schema，自动解析为结构体
	// schema参数应该是结构体指针，会被自动填充
	GenerateStructured(ctx context.Context, prompt string, schema interface{}, opts ...LLMOption) error

	// IsEnabled 检查LLM功能是否启用
	// 用于判断LLM功能是否可用
	IsEnabled() bool

	// GetProviderName 获取当前Provider名称
	// 返回当前使用的Provider（openai, anthropic等）
	GetProviderName() string
}

// LLMOption LLM选项函数类型
// 使用函数式选项模式配置LLM调用参数
type LLMOption func(*LLMOptions)

// LLMOptions LLM配置选项
type LLMOptions struct {
	Model        string  // 模型名称
	Temperature  float32 // 生成温度 0.0-2.0
	MaxTokens    int     // 最大token数
	SystemPrompt string  // 系统提示词
}

// WithLLMModel 设置模型
func WithLLMModel(model string) LLMOption {
	return func(opts *LLMOptions) {
		opts.Model = model
	}
}

// WithLLMTemperature 设置温度
func WithLLMTemperature(t float32) LLMOption {
	return func(opts *LLMOptions) {
		opts.Temperature = t
	}
}

// WithLLMMaxTokens 设置最大token数
func WithLLMMaxTokens(n int) LLMOption {
	return func(opts *LLMOptions) {
		opts.MaxTokens = n
	}
}

// WithLLMSystemPrompt 设置系统提示
func WithLLMSystemPrompt(prompt string) LLMOption {
	return func(opts *LLMOptions) {
		opts.SystemPrompt = prompt
	}
}
