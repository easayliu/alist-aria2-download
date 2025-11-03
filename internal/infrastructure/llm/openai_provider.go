package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/openai"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// OpenAIProvider OpenAI提供商实现
// 实现了Provider接口，封装了OpenAI API调用
type OpenAIProvider struct {
	config     *config.OpenAIConfig // OpenAI配置
	client     *openai.Client       // OpenAI底层客户端
	chatClient *openai.ChatClient   // Chat专用客户端
}

// NewOpenAIProvider 创建OpenAI Provider
// 根据配置初始化OpenAI客户端
func NewOpenAIProvider(cfg *config.OpenAIConfig) (*OpenAIProvider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("OpenAI config cannot be nil")
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API Key cannot be empty")
	}

	// 构建OpenAI客户端配置
	clientConfig := openai.NewConfigFromAppConfig(cfg)

	// 验证配置
	if err := clientConfig.Validate(); err != nil {
		return nil, fmt.Errorf("OpenAI config validation failed: %w", err)
	}

	// 创建客户端
	client, err := openai.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI client: %w", err)
	}
	chatClient := openai.NewChatClient(client)

	return &OpenAIProvider{
		config:     cfg,
		client:     client,
		chatClient: chatClient,
	}, nil
}

// Name 返回Provider名称
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Generate 生成文本
// 实现Provider接口，提供非流式文本生成
func (p *OpenAIProvider) Generate(ctx context.Context, prompt string, opts ...Option) (string, error) {
	// 构建默认选项
	defaults := &GenerateOptions{
		Model:       p.config.Model,
		Temperature: p.config.Temperature,
		MaxTokens:   p.config.MaxTokens,
	}

	// 应用用户选项
	options := ApplyOptions(defaults, opts...)

	// 构建消息列表
	messages := p.buildMessages(prompt, options)

	// 构建ChatOption
	chatOpts := []openai.ChatOption{
		openai.WithModel(options.Model),
		openai.WithTemperature(options.Temperature),
		openai.WithMaxTokens(options.MaxTokens),
	}

	// 如果context没有deadline,使用配置中的timeout创建一个带超时的context
	var apiCtx context.Context
	var cancel context.CancelFunc

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		apiCtx, cancel = context.WithTimeout(ctx, p.client.Config().Timeout)
		defer cancel()
	} else {
		apiCtx = ctx
	}

	// 调用OpenAI API (使用带timeout的context)
	resp, err := p.chatClient.Complete(apiCtx, messages, chatOpts...)
	if err != nil {
		return "", fmt.Errorf("OpenAI生成失败: %w", err)
	}

	// 提取响应内容
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("OpenAI响应中没有choices")
	}

	return resp.Choices[0].Message.Content, nil
}

// GenerateStream 流式生成文本
// 实现Provider接口，提供流式文本生成
func (p *OpenAIProvider) GenerateStream(ctx context.Context, prompt string, opts ...Option) (<-chan string, <-chan error) {
	// 构建默认选项
	defaults := &GenerateOptions{
		Model:       p.config.Model,
		Temperature: p.config.Temperature,
		MaxTokens:   p.config.MaxTokens,
	}

	// 应用用户选项
	options := ApplyOptions(defaults, opts...)

	// 构建消息列表
	messages := p.buildMessages(prompt, options)

	// 构建ChatOption
	chatOpts := []openai.ChatOption{
		openai.WithModel(options.Model),
		openai.WithTemperature(options.Temperature),
		openai.WithMaxTokens(options.MaxTokens),
	}

	// 调用OpenAI流式API
	return p.chatClient.CompleteStream(ctx, messages, chatOpts...)
}

// GenerateStructured 生成结构化输出（JSON）
// 使用OpenAI的JSON mode实现结构化输出
func (p *OpenAIProvider) GenerateStructured(ctx context.Context, prompt string, schema interface{}, opts ...Option) error {
	// 构建默认选项
	defaults := &GenerateOptions{
		Model:       p.config.Model,
		Temperature: p.config.Temperature,
		MaxTokens:   p.config.MaxTokens,
		JSONMode:    true, // 强制启用JSON模式
	}

	// 应用用户选项
	options := ApplyOptions(defaults, opts...)

	// 在提示词中添加JSON格式要求
	enhancedPrompt := prompt + "\n\n请以JSON格式返回结果。"
	if options.SystemPrompt != "" {
		enhancedPrompt = options.SystemPrompt + "\n\n" + enhancedPrompt
	}

	// 构建消息
	messages := []openai.ChatMessage{
		{
			Role:    "user",
			Content: enhancedPrompt,
		},
	}

	// 如果有对话历史，添加到消息列表
	if len(options.Messages) > 0 {
		var msgs []openai.ChatMessage
		for _, msg := range options.Messages {
			msgs = append(msgs, openai.ChatMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
		msgs = append(msgs, messages...)
		messages = msgs
	}

	// 构建请求
	req := &openai.ChatRequest{
		Model:       options.Model,
		Messages:    messages,
		Temperature: options.Temperature,
		MaxTokens:   options.MaxTokens,
		ResponseFormat: &openai.ResponseFormat{
			Type: "json_object", // 启用JSON模式
		},
	}

	// 记录请求信息
	logger.Debug("OpenAI structured request",
		"model", req.Model,
		"temperature", req.Temperature,
		"maxTokens", req.MaxTokens,
		"messageCount", len(req.Messages))

	// 只在Debug级别记录完整的提示词(可能很长)
	logger.Debug("OpenAI request prompt", "prompt", enhancedPrompt)

	// 如果context没有deadline,使用配置中的timeout创建一个带超时的context
	// 确保API调用不会无限期挂起
	var apiCtx context.Context
	var cancel context.CancelFunc

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		// Context没有deadline,使用配置的timeout
		apiCtx, cancel = context.WithTimeout(ctx, p.client.Config().Timeout)
		defer cancel()
		logger.Debug("Using configured timeout for API call",
			"timeout", p.client.Config().Timeout)
	} else {
		// Context已有deadline,直接使用
		apiCtx = ctx
	}

	// 调用OpenAI API (使用带timeout的context)
	resp, err := p.client.ChatCompletion(apiCtx, req)
	if err != nil {
		logger.Error("OpenAI structured generation failed",
			"model", req.Model,
			"error", err)
		return fmt.Errorf("OpenAI结构化生成失败: %w", err)
	}

	// 提取响应内容
	if len(resp.Choices) == 0 {
		logger.Error("OpenAI response has no choices", "response", resp)
		return fmt.Errorf("OpenAI响应中没有choices")
	}

	content := resp.Choices[0].Message.Content

	// 记录响应信息
	logger.Info("OpenAI structured response received",
		"model", resp.Model,
		"finishReason", resp.Choices[0].FinishReason,
		"promptTokens", resp.Usage.PromptTokens,
		"completionTokens", resp.Usage.CompletionTokens,
		"totalTokens", resp.Usage.TotalTokens)

	// 记录原始响应内容
	logger.Debug("OpenAI raw response content", "content", content)

	// 清理可能的代码块标记（某些模型会返回```json...```格式）
	content = cleanJSONContent(content)

	// 预处理JSON:将空字符串转为null(兼容不同LLM的输出)
	content = normalizeJSONEmptyValues(content)

	// 记录清理后的内容
	logger.Debug("OpenAI cleaned JSON content", "content", content)

	// 解析JSON到schema
	if err := json.Unmarshal([]byte(content), schema); err != nil {
		logger.Error("Failed to parse JSON response",
			"error", err,
			"rawContent", content)
		return fmt.Errorf("解析JSON响应失败: %w, 原始内容: %s", err, content)
	}

	logger.Debug("OpenAI JSON parsed successfully")
	return nil
}

// cleanJSONContent 清理JSON内容中的代码块标记
func cleanJSONContent(content string) string {
	// 移除开头的```json或```
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
	}

	// 移除结尾的```
	content = strings.TrimSpace(content)
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
	}

	return strings.TrimSpace(content)
}

// normalizeJSONEmptyValues 标准化JSON中的空值
// 将数字字段的空字符串 "" 转为 null,避免JSON解析错误
func normalizeJSONEmptyValues(content string) string {
	// 处理常见的数字字段空字符串
	// "year": "" -> "year": null
	// "season": "" -> "season": null
	// "episode": "" -> "episode": null

	// 使用正则替换 "字段名": "" 为 "字段名": null
	numericFields := []string{"year", "season", "episode"}

	for _, field := range numericFields {
		// 匹配 "field": "" 模式(考虑空格)
		pattern := fmt.Sprintf(`"%s"\s*:\s*""`, field)
		replacement := fmt.Sprintf(`"%s": null`, field)
		content = regexp.MustCompile(pattern).ReplaceAllString(content, replacement)
	}

	return content
}

// IsAvailable 检查Provider是否可用
// 简单的健康检查，实际场景可以ping OpenAI API
func (p *OpenAIProvider) IsAvailable() bool {
	return p.config.APIKey != "" && p.client != nil
}

// buildMessages 构建ChatMessage列表
// 将GenerateOptions转换为OpenAI的ChatMessage格式
func (p *OpenAIProvider) buildMessages(prompt string, opts *GenerateOptions) []openai.ChatMessage {
	var messages []openai.ChatMessage

	// 添加系统提示词（如果有）
	if opts.SystemPrompt != "" {
		messages = append(messages, openai.ChatMessage{
			Role:    "system",
			Content: opts.SystemPrompt,
		})
	}

	// 添加对话历史（如果有）
	if len(opts.Messages) > 0 {
		for _, msg := range opts.Messages {
			messages = append(messages, openai.ChatMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// 添加用户提示词
	messages = append(messages, openai.ChatMessage{
		Role:    "user",
		Content: prompt,
	})

	return messages
}
