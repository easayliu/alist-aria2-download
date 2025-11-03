package llm

import (
	"context"
	"errors"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
)

var ErrLLMDisabled = errors.New("LLM功能未启用")

// DisabledLLMService 禁用的LLM服务
// 当LLM功能未配置或初始化失败时，使用此服务提供优雅降级
// 所有方法调用都会返回ErrLLMDisabled错误
type DisabledLLMService struct{}

// NewDisabledLLMService 创建禁用的LLM服务
func NewDisabledLLMService() contracts.LLMService {
	return &DisabledLLMService{}
}

// GenerateText 生成文本（禁用状态）
// 返回ErrLLMDisabled错误
func (s *DisabledLLMService) GenerateText(ctx context.Context, prompt string, opts ...contracts.LLMOption) (string, error) {
	return "", ErrLLMDisabled
}

// GenerateTextStream 流式生成文本（禁用状态）
// 返回立即关闭的通道和ErrLLMDisabled错误
func (s *DisabledLLMService) GenerateTextStream(ctx context.Context, prompt string, opts ...contracts.LLMOption) (<-chan string, <-chan error) {
	textChan := make(chan string)
	errChan := make(chan error, 1)
	close(textChan)
	errChan <- ErrLLMDisabled
	close(errChan)
	return textChan, errChan
}

// GenerateStructured 生成结构化输出（禁用状态）
// 返回ErrLLMDisabled错误
func (s *DisabledLLMService) GenerateStructured(ctx context.Context, prompt string, schema interface{}, opts ...contracts.LLMOption) error {
	return ErrLLMDisabled
}

// IsEnabled 检查LLM功能是否启用
// 始终返回false
func (s *DisabledLLMService) IsEnabled() bool {
	return false
}

// GetProviderName 获取Provider名称
// 返回"disabled"表示服务已禁用
func (s *DisabledLLMService) GetProviderName() string {
	return "disabled"
}
