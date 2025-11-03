package openai

import (
	"os"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
)

// Config OpenAI客户端配置
type Config struct {
	APIKey      string        // API密钥
	BaseURL     string        // API基础URL
	Model       string        // 默认模型
	Temperature float32       // 默认温度参数
	MaxTokens   int           // 默认最大Token数
	Timeout     time.Duration // HTTP超时时间
	QPS         int           // 每秒请求数限制
}

// NewConfigFromAppConfig 从应用配置创建OpenAI客户端配置
// 优先使用环境变量中的API Key
func NewConfigFromAppConfig(cfg *config.OpenAIConfig) *Config {
	apiKey := cfg.APIKey
	// 支持从环境变量读取API Key
	if envKey := os.Getenv("OPENAI_API_KEY"); envKey != "" {
		apiKey = envKey
	}

	return &Config{
		APIKey:      apiKey,
		BaseURL:     cfg.BaseURL,
		Model:       cfg.Model,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
		Timeout:     time.Duration(cfg.Timeout) * time.Second,
		QPS:         cfg.QPS,
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return ErrMissingAPIKey
	}
	if c.BaseURL == "" {
		c.BaseURL = "https://api.openai.com/v1"
	}
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	if c.Model == "" {
		c.Model = "gpt-3.5-turbo"
	}
	return nil
}
