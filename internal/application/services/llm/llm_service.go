package llm

import (
	"context"
	"fmt"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/llm"
)

// AppLLMService LLM应用服务实现
// 实现contracts.LLMService接口，提供应用层的LLM调用能力
type AppLLMService struct {
	config    *config.LLMConfig // LLM配置
	llmClient *llm.Client       // LLM基础设施层客户端
	factory   *llm.Factory      // LLM工厂
}

// NewAppLLMService 创建LLM应用服务
// 根据配置初始化LLM服务，创建底层客户端
func NewAppLLMService(cfg *config.LLMConfig) (contracts.LLMService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("LLM配置不能为nil")
	}

	// 创建工厂
	factory := llm.NewFactory(cfg)

	// 如果未启用，返回一个禁用的服务
	if !cfg.Enabled {
		return &AppLLMService{
			config:  cfg,
			factory: factory,
		}, nil
	}

	// 验证配置
	if err := factory.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("LLM配置验证失败: %w", err)
	}

	// 创建LLM客户端
	client, err := factory.CreateClient()
	if err != nil {
		return nil, fmt.Errorf("创建LLM客户端失败: %w", err)
	}

	return &AppLLMService{
		config:    cfg,
		llmClient: client,
		factory:   factory,
	}, nil
}

// GenerateText 生成文本
// 实现contracts.LLMService接口
func (s *AppLLMService) GenerateText(ctx context.Context, prompt string, opts ...contracts.LLMOption) (string, error) {
	if !s.IsEnabled() {
		return "", fmt.Errorf("LLM功能未启用")
	}

	// 转换应用层选项到基础设施层选项
	llmOpts := s.convertOptions(opts)

	// 调用基础设施层
	return s.llmClient.Generate(ctx, prompt, llmOpts...)
}

// GenerateTextStream 流式生成文本
// 实现contracts.LLMService接口
func (s *AppLLMService) GenerateTextStream(ctx context.Context, prompt string, opts ...contracts.LLMOption) (<-chan string, <-chan error) {
	if !s.IsEnabled() {
		errChan := make(chan error, 1)
		textChan := make(chan string)
		errChan <- fmt.Errorf("LLM功能未启用")
		close(textChan)
		close(errChan)
		return textChan, errChan
	}

	// 转换应用层选项到基础设施层选项
	llmOpts := s.convertOptions(opts)

	// 调用基础设施层
	return s.llmClient.GenerateStream(ctx, prompt, llmOpts...)
}

// GenerateStructured 生成结构化输出
// 实现contracts.LLMService接口
func (s *AppLLMService) GenerateStructured(ctx context.Context, prompt string, schema interface{}, opts ...contracts.LLMOption) error {
	if !s.IsEnabled() {
		return fmt.Errorf("LLM功能未启用")
	}

	// 转换应用层选项到基础设施层选项
	llmOpts := s.convertOptions(opts)

	// 调用基础设施层
	return s.llmClient.GenerateStructured(ctx, prompt, schema, llmOpts...)
}

// IsEnabled 检查LLM功能是否启用
// 实现contracts.LLMService接口
func (s *AppLLMService) IsEnabled() bool {
	return s.config.Enabled && s.llmClient != nil && s.llmClient.IsAvailable()
}

// GetProviderName 获取当前Provider名称
// 实现contracts.LLMService接口
func (s *AppLLMService) GetProviderName() string {
	if s.llmClient == nil {
		return s.factory.GetProviderName()
	}
	return s.llmClient.ProviderName()
}

// convertOptions 转换应用层选项到基础设施层选项
// 将contracts.LLMOption转换为llm.Option
func (s *AppLLMService) convertOptions(opts []contracts.LLMOption) []llm.Option {
	// 构建应用层选项
	appOpts := &contracts.LLMOptions{}
	for _, opt := range opts {
		opt(appOpts)
	}

	// 转换为基础设施层选项
	var llmOpts []llm.Option

	if appOpts.Model != "" {
		llmOpts = append(llmOpts, llm.WithModel(appOpts.Model))
	}

	if appOpts.Temperature > 0 {
		llmOpts = append(llmOpts, llm.WithTemperature(appOpts.Temperature))
	}

	if appOpts.MaxTokens > 0 {
		llmOpts = append(llmOpts, llm.WithMaxTokens(appOpts.MaxTokens))
	}

	if appOpts.SystemPrompt != "" {
		llmOpts = append(llmOpts, llm.WithSystemPrompt(appOpts.SystemPrompt))
	}

	return llmOpts
}
