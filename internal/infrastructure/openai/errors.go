package openai

import "errors"

var (
	// ErrMissingAPIKey API密钥缺失错误
	ErrMissingAPIKey = errors.New("OpenAI API密钥不能为空")

	// ErrInvalidConfig 配置无效错误
	ErrInvalidConfig = errors.New("OpenAI配置无效")

	// ErrRequestFailed 请求失败错误
	ErrRequestFailed = errors.New("OpenAI API请求失败")

	// ErrRateLimitExceeded 速率限制超出错误
	ErrRateLimitExceeded = errors.New("超出OpenAI API速率限制")

	// ErrEmptyResponse 空响应错误
	ErrEmptyResponse = errors.New("OpenAI API返回空响应")

	// ErrContextCanceled 上下文取消错误
	ErrContextCanceled = errors.New("请求被取消")
)
