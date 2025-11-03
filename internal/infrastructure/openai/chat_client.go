package openai

import (
	"context"
	"fmt"
)

// ChatClient Chat专用客户端
// 提供更简洁的Chat Completion API封装
type ChatClient struct {
	client *Client // 底层OpenAI客户端
}

// NewChatClient 创建Chat客户端
func NewChatClient(client *Client) *ChatClient {
	if client == nil {
		panic("client不能为nil")
	}
	return &ChatClient{
		client: client,
	}
}

// Complete 执行非流式Chat请求
// 返回完整的响应结果
//
// 使用示例:
//   messages := []ChatMessage{
//       {Role: "user", Content: "你好"},
//   }
//   resp, err := chatClient.Complete(ctx, messages, WithTemperature(0.8))
func (c *ChatClient) Complete(ctx context.Context, messages []ChatMessage, opts ...ChatOption) (*ChatResponse, error) {
	// 构建请求
	req := &ChatRequest{
		Model:       c.client.config.Model,
		Messages:    messages,
		Temperature: c.client.config.Temperature,
		MaxTokens:   c.client.config.MaxTokens,
	}

	// 应用选项
	for _, opt := range opts {
		opt(req)
	}

	// 执行请求
	return c.client.ChatCompletion(ctx, req)
}

// CompleteStream 执行流式Chat请求
// 返回文本流Channel，实时输出生成的文本
//
// 使用示例:
//   textChan, errChan := chatClient.CompleteStream(ctx, messages)
//   for text := range textChan {
//       fmt.Print(text) // 实时打印生成的文本
//   }
//   if err := <-errChan; err != nil {
//       log.Fatal(err)
//   }
func (c *ChatClient) CompleteStream(ctx context.Context, messages []ChatMessage, opts ...ChatOption) (<-chan string, <-chan error) {
	textChan := make(chan string, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(textChan)
		defer close(errChan)

		// 构建请求
		req := &ChatRequest{
			Model:       c.client.config.Model,
			Messages:    messages,
			Temperature: c.client.config.Temperature,
			MaxTokens:   c.client.config.MaxTokens,
		}

		// 应用选项
		for _, opt := range opts {
			opt(req)
		}

		// 执行流式请求
		handler, err := c.client.ChatCompletionStream(ctx, req)
		if err != nil {
			errChan <- fmt.Errorf("创建流式请求失败: %w", err)
			return
		}

		// 读取流式响应
		chunkChan, chunkErrChan := handler.ReadStream(ctx)

		// 处理流式数据
		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return

			case chunk, ok := <-chunkChan:
				if !ok {
					// 流已结束
					return
				}

				// 提取文本内容
				content := ExtractContent(chunk)
				if content != "" {
					select {
					case <-ctx.Done():
						errChan <- ctx.Err()
						return
					case textChan <- content:
						// 成功发送
					}
				}

				// 检查是否完成
				if IsFinished(chunk) {
					return
				}

			case err, ok := <-chunkErrChan:
				if ok && err != nil {
					errChan <- err
					return
				}
			}
		}
	}()

	return textChan, errChan
}

// CompleteWithContext 执行非流式Chat请求（带Context）
// 提供更灵活的Context控制
func (c *ChatClient) CompleteWithContext(ctx context.Context, messages []ChatMessage, opts ...ChatOption) (*ChatResponse, error) {
	return c.Complete(ctx, messages, opts...)
}

// ChatOption 配置选项函数
type ChatOption func(*ChatRequest)

// WithTemperature 设置温度参数
// temperature: 0.0-2.0，越高越随机
func WithTemperature(temperature float32) ChatOption {
	return func(req *ChatRequest) {
		req.Temperature = temperature
	}
}

// WithMaxTokens 设置最大token数
func WithMaxTokens(maxTokens int) ChatOption {
	return func(req *ChatRequest) {
		req.MaxTokens = maxTokens
	}
}

// WithModel 设置使用的模型
func WithModel(model string) ChatOption {
	return func(req *ChatRequest) {
		req.Model = model
	}
}

// WithTopP 设置核采样参数
// topP: 0.0-1.0，控制生成的多样性
func WithTopP(topP float32) ChatOption {
	return func(req *ChatRequest) {
		req.TopP = topP
	}
}

// SimpleComplete 简化的完成方法，直接传入用户消息
// 返回助手的回复文本
//
// 使用示例:
//   reply, err := chatClient.SimpleComplete(ctx, "你好，介绍一下自己")
func (c *ChatClient) SimpleComplete(ctx context.Context, userMessage string, opts ...ChatOption) (string, error) {
	messages := []ChatMessage{
		{Role: "user", Content: userMessage},
	}

	resp, err := c.Complete(ctx, messages, opts...)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("响应中没有choices")
	}

	return resp.Choices[0].Message.Content, nil
}

// SimpleCompleteStream 简化的流式完成方法
// 使用示例:
//   textChan, errChan := chatClient.SimpleCompleteStream(ctx, "讲个故事")
func (c *ChatClient) SimpleCompleteStream(ctx context.Context, userMessage string, opts ...ChatOption) (<-chan string, <-chan error) {
	messages := []ChatMessage{
		{Role: "user", Content: userMessage},
	}

	return c.CompleteStream(ctx, messages, opts...)
}
