package llm

import (
	"fmt"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
)

// Factory LLM工厂
// 使用工厂模式创建不同的Provider和Client
// 支持多Provider策略，便于扩展和切换
type Factory struct {
	config *config.LLMConfig // LLM总配置
}

// NewFactory 创建LLM工厂
// config: LLM总配置，包含所有Provider的配置
func NewFactory(cfg *config.LLMConfig) *Factory {
	if cfg == nil {
		panic("LLM配置不能为nil")
	}

	return &Factory{
		config: cfg,
	}
}

// CreateProvider 创建指定的Provider
// 根据providerName创建对应的Provider实例
//
// 参数:
//   - providerName: Provider名称（openai, anthropic, ollama等）
//
// 返回:
//   - Provider: Provider实例
//   - error: 错误信息
//
// 支持的Provider:
//   - openai: OpenAI (GPT-3.5, GPT-4等)
//   - anthropic: Anthropic (Claude系列) [预留]
//   - ollama: Ollama (本地部署) [预留]
func (f *Factory) CreateProvider(providerName string) (Provider, error) {
	switch providerName {
	case "openai":
		return f.createOpenAIProvider()

	case "anthropic":
		// 预留: 未来实现Anthropic Provider
		return nil, fmt.Errorf("anthropic provider尚未实现，敬请期待")

	case "ollama":
		// 预留: 未来实现Ollama Provider
		return nil, fmt.Errorf("ollama provider尚未实现，敬请期待")

	default:
		return nil, fmt.Errorf("未知的provider: %s，支持的provider: openai", providerName)
	}
}

// CreateDefaultProvider 创建默认Provider
// 根据配置中的provider字段创建Provider
func (f *Factory) CreateDefaultProvider() (Provider, error) {
	providerName := f.config.Provider
	if providerName == "" {
		providerName = "openai" // 默认使用OpenAI
	}

	return f.CreateProvider(providerName)
}

// CreateClient 创建LLM客户端
// 使用默认Provider创建统一的LLM客户端
//
// 返回:
//   - *Client: LLM统一客户端
//   - error: 错误信息
//
// 使用示例:
//   factory := NewFactory(config)
//   client, err := factory.CreateClient()
//   if err != nil {
//       log.Fatal(err)
//   }
//   text, err := client.Generate(ctx, "你好")
func (f *Factory) CreateClient() (*Client, error) {
	provider, err := f.CreateDefaultProvider()
	if err != nil {
		return nil, fmt.Errorf("创建Provider失败: %w", err)
	}

	return NewClient(provider), nil
}

// CreateClientWithProvider 使用指定Provider创建客户端
// 参数:
//   - providerName: Provider名称
//
// 返回:
//   - *Client: LLM统一客户端
//   - error: 错误信息
func (f *Factory) CreateClientWithProvider(providerName string) (*Client, error) {
	provider, err := f.CreateProvider(providerName)
	if err != nil {
		return nil, fmt.Errorf("创建Provider失败: %w", err)
	}

	return NewClient(provider), nil
}

// IsEnabled 检查LLM功能是否启用
// 根据配置判断LLM功能是否开启
func (f *Factory) IsEnabled() bool {
	return f.config.Enabled
}

// GetProviderName 获取当前配置的Provider名称
func (f *Factory) GetProviderName() string {
	if f.config.Provider == "" {
		return "openai" // 默认
	}
	return f.config.Provider
}

// createOpenAIProvider 创建OpenAI Provider（内部方法）
func (f *Factory) createOpenAIProvider() (Provider, error) {
	// 验证OpenAI配置
	if f.config.OpenAI.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API Key未配置，请在配置文件中设置 llm.openai.api_key")
	}

	// 创建OpenAI Provider
	provider, err := NewOpenAIProvider(&f.config.OpenAI)
	if err != nil {
		return nil, fmt.Errorf("创建OpenAI Provider失败: %w", err)
	}

	return provider, nil
}

// ValidateConfig 验证配置有效性
// 在创建Provider之前验证配置
func (f *Factory) ValidateConfig() error {
	if !f.config.Enabled {
		return fmt.Errorf("LLM功能未启用")
	}

	providerName := f.GetProviderName()

	switch providerName {
	case "openai":
		if f.config.OpenAI.APIKey == "" {
			return fmt.Errorf("OpenAI API Key未配置")
		}
		if f.config.OpenAI.Model == "" {
			return fmt.Errorf("OpenAI Model未配置")
		}

	case "anthropic":
		return fmt.Errorf("anthropic provider尚未实现")

	case "ollama":
		return fmt.Errorf("ollama provider尚未实现")

	default:
		return fmt.Errorf("未知的provider: %s", providerName)
	}

	return nil
}
