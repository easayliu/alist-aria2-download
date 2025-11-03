package openai_test

import (
	"context"
	"testing"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/openai"
)

// 示例：如何创建和使用OpenAI客户端
func ExampleNewClient() {
	// 创建配置
	cfg := &openai.Config{
		APIKey:      "sk-xxx", // 实际使用时从环境变量获取
		BaseURL:     "https://api.openai.com/v1",
		Model:       "gpt-3.5-turbo",
		Temperature: 0.3,
		MaxTokens:   1000,
		Timeout:     30 * time.Second,
		QPS:         10,
	}

	// 创建客户端
	client, err := openai.NewClient(cfg)
	if err != nil {
		panic(err)
	}

	// 使用客户端
	_ = client
}

// 示例：非流式Chat请求
func ExampleClient_ChatCompletion() {
	cfg := &openai.Config{
		APIKey:  "sk-xxx",
		Model:   "gpt-3.5-turbo",
		Timeout: 30 * time.Second,
		QPS:     10,
	}

	client, _ := openai.NewClient(cfg)

	// 构建请求
	req := &openai.ChatRequest{
		Model: "gpt-3.5-turbo",
		Messages: []openai.ChatMessage{
			{Role: "user", Content: "你好"},
		},
		Temperature: 0.7,
	}

	// 发送请求
	ctx := context.Background()
	resp, err := client.ChatCompletion(ctx, req)
	if err != nil {
		panic(err)
	}

	// 使用响应
	_ = resp
}

// 示例：使用ChatClient的简化API
func ExampleChatClient_SimpleComplete() {
	cfg := &openai.Config{
		APIKey:  "sk-xxx",
		Model:   "gpt-3.5-turbo",
		Timeout: 30 * time.Second,
		QPS:     10,
	}

	client, _ := openai.NewClient(cfg)
	chatClient := openai.NewChatClient(client)

	// 简单调用
	ctx := context.Background()
	reply, err := chatClient.SimpleComplete(ctx, "介绍一下Go语言")
	if err != nil {
		panic(err)
	}

	// 使用回复
	_ = reply
}

// 示例：流式响应
func ExampleChatClient_SimpleCompleteStream() {
	cfg := &openai.Config{
		APIKey:  "sk-xxx",
		Model:   "gpt-3.5-turbo",
		Timeout: 30 * time.Second,
		QPS:     10,
	}

	client, _ := openai.NewClient(cfg)
	chatClient := openai.NewChatClient(client)

	// 流式调用
	ctx := context.Background()
	textChan, errChan := chatClient.SimpleCompleteStream(ctx, "讲个故事")

	// 实时处理文本
	for text := range textChan {
		_ = text // 打印或处理文本片段
	}

	// 检查错误
	if err := <-errChan; err != nil {
		panic(err)
	}
}

// 非示例测试，仅用于验证代码编译
func TestPlaceholder(t *testing.T) {
	// 占位测试，确保包可以编译
}
