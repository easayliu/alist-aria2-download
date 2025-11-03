package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/ratelimit"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// Client OpenAI API客户端
type Client struct {
	config      *Config               // 配置
	httpClient  *http.Client          // HTTP客户端
	rateLimiter *ratelimit.RateLimiter // 速率限制器
}

// NewClient 创建OpenAI客户端
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		return nil, ErrInvalidConfig
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// 创建速率限制器
	limiter := ratelimit.NewRateLimiter(config.QPS)

	logger.Info("Creating OpenAI client",
		"base_url", config.BaseURL,
		"model", config.Model,
		"qps", config.QPS,
		"timeout", config.Timeout,
	)

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		rateLimiter: limiter,
	}, nil
}

// ChatCompletion 执行Chat Completion请求（非流式）
func (c *Client) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// 速率限制
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("速率限制等待失败: %w", err)
	}

	// 确保非流式
	req.Stream = false

	// 序列化请求
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	logger.Debug("发送OpenAI Chat请求",
		"model", req.Model,
		"messages_count", len(req.Messages),
		"temperature", req.Temperature,
	)

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.config.BaseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	c.setHeaders(httpReq)

	// 发送请求
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	// 解析响应
	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	logger.Debug("OpenAI Chat响应成功",
		"id", chatResp.ID,
		"choices_count", len(chatResp.Choices),
		"total_tokens", chatResp.Usage.TotalTokens,
	)

	return &chatResp, nil
}

// ChatCompletionStream 执行Chat Completion流式请求
// 返回StreamHandler用于读取流式响应
func (c *Client) ChatCompletionStream(ctx context.Context, req *ChatRequest) (*StreamHandler, error) {
	// 速率限制
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("速率限制等待失败: %w", err)
	}

	// 确保启用流式
	req.Stream = true

	// 序列化请求
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	logger.Debug("发送OpenAI流式Chat请求",
		"model", req.Model,
		"messages_count", len(req.Messages),
	)

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.config.BaseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	c.setHeaders(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")

	// 发送请求（使用无超时的客户端，因为流式响应可能很长）
	client := &http.Client{
		Timeout: 0, // 流式请求不设置超时，由Context控制
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("发送HTTP请求失败: %w", err)
	}

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, c.handleErrorResponse(resp)
	}

	logger.Debug("OpenAI流式Chat响应已建立连接")

	// 创建流式处理器（注意：调用者负责关闭响应体）
	handler := NewStreamHandler(resp.Body)
	return handler, nil
}

// setHeaders 设置请求头
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
}

// handleErrorResponse 处理错误响应
func (c *Client) handleErrorResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("HTTP %d: 读取错误响应失败: %w", resp.StatusCode, err)
	}

	// 尝试解析为OpenAI错误格式
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		return fmt.Errorf("OpenAI API错误 (%s): %s", errResp.Error.Type, errResp.Error.Message)
	}

	// 返回原始错误
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
}

// Config 返回客户端配置（只读）
func (c *Client) Config() *Config {
	return c.config
}
