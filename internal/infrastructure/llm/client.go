package llm

import (
	"context"
	"fmt"
)

// Client LLM统一客户端
// 提供统一的LLM调用接口，屏蔽底层Provider差异
// 使用门面模式（Facade Pattern）简化调用
type Client struct {
	provider Provider // 底层LLM Provider
}

// NewClient 创建LLM客户端
// provider: 底层Provider实现（如OpenAIProvider）
func NewClient(provider Provider) *Client {
	if provider == nil {
		panic("provider不能为nil")
	}

	return &Client{
		provider: provider,
	}
}

// Generate 生成文本
// 统一的文本生成接口，支持所有Provider
//
// 参数:
//   - ctx: 上下文，用于控制超时和取消
//   - prompt: 用户提示词
//   - opts: 可选配置参数
//
// 返回:
//   - string: 生成的文本内容
//   - error: 错误信息
//
// 使用示例:
//
//	client := llm.NewClient(provider)
//	text, err := client.Generate(ctx, "你好", llm.WithTemperature(0.7))
func (c *Client) Generate(ctx context.Context, prompt string, opts ...Option) (string, error) {
	if !c.provider.IsAvailable() {
		return "", fmt.Errorf("provider %s 不可用", c.provider.Name())
	}

	return c.provider.Generate(ctx, prompt, opts...)
}

// GenerateStream 流式生成文本
// 统一的流式文本生成接口，支持所有Provider
//
// 参数:
//   - ctx: 上下文
//   - prompt: 用户提示词
//   - opts: 可选配置参数
//
// 返回:
//   - <-chan string: 文本流通道
//   - <-chan error: 错误通道
//
// 使用示例:
//
//	textChan, errChan := client.GenerateStream(ctx, "讲个故事")
//	for text := range textChan {
//	    fmt.Print(text)
//	}
//	if err := <-errChan; err != nil {
//	    log.Fatal(err)
//	}
func (c *Client) GenerateStream(ctx context.Context, prompt string, opts ...Option) (<-chan string, <-chan error) {
	if !c.provider.IsAvailable() {
		errChan := make(chan error, 1)
		textChan := make(chan string)
		errChan <- fmt.Errorf("provider %s 不可用", c.provider.Name())
		close(textChan)
		close(errChan)
		return textChan, errChan
	}

	return c.provider.GenerateStream(ctx, prompt, opts...)
}

// GenerateStructured 生成结构化输出
// 统一的结构化输出接口，支持所有Provider
//
// 参数:
//   - ctx: 上下文
//   - prompt: 用户提示词
//   - schema: 期望的输出结构，将被填充数据
//   - opts: 可选配置参数
//
// 返回:
//   - error: 错误信息，成功时schema会被填充数据
//
// 使用示例:
//
//	type Result struct {
//	    Title string `json:"title"`
//	    Year  int    `json:"year"`
//	}
//	var result Result
//	err := client.GenerateStructured(ctx, "分析这个文件名", &result)
func (c *Client) GenerateStructured(ctx context.Context, prompt string, schema interface{}, opts ...Option) error {
	if !c.provider.IsAvailable() {
		return fmt.Errorf("provider %s 不可用", c.provider.Name())
	}

	return c.provider.GenerateStructured(ctx, prompt, schema, opts...)
}

// GetProvider 获取当前Provider
// 返回底层Provider实例，用于高级用法
func (c *Client) GetProvider() Provider {
	return c.provider
}

// IsAvailable 检查客户端是否可用
// 委托给底层Provider检查
func (c *Client) IsAvailable() bool {
	return c.provider.IsAvailable()
}

// ProviderName 获取Provider名称
// 返回当前使用的Provider名称
func (c *Client) ProviderName() string {
	return c.provider.Name()
}
