package llm

import (
	"context"
	"fmt"
	"strings"
)

// StreamClient 流式客户端接口
// 提供Provider无关的流式文本生成能力
type StreamClient interface {
	// StreamGenerate 流式生成文本
	// 返回文本流Channel和错误Channel
	StreamGenerate(ctx context.Context, prompt string, opts ...Option) (<-chan string, <-chan error)
}

// StreamResponse 流式响应包装
// 提供便捷的流式数据处理方法
type StreamResponse struct {
	TextChan  <-chan string          // 文本流Channel
	ErrorChan <-chan error           // 错误Channel
	Cancel    context.CancelFunc     // 取消函数
	ctx       context.Context        // 关联的Context
}

// NewStreamResponse 创建流式响应
func NewStreamResponse(textChan <-chan string, errChan <-chan error, cancel context.CancelFunc) *StreamResponse {
	return &StreamResponse{
		TextChan:  textChan,
		ErrorChan: errChan,
		Cancel:    cancel,
	}
}

// NewStreamResponseWithContext 创建带Context的流式响应
func NewStreamResponseWithContext(ctx context.Context, textChan <-chan string, errChan <-chan error, cancel context.CancelFunc) *StreamResponse {
	return &StreamResponse{
		TextChan:  textChan,
		ErrorChan: errChan,
		Cancel:    cancel,
		ctx:       ctx,
	}
}

// Collect 收集所有流式输出为完整文本
// 阻塞直到流结束或发生错误
//
// 使用示例:
//   resp := NewStreamResponse(textChan, errChan, cancel)
//   fullText, err := resp.Collect()
func (r *StreamResponse) Collect() (string, error) {
	var builder strings.Builder

	for {
		select {
		case text, ok := <-r.TextChan:
			if !ok {
				// Channel已关闭，返回累积的文本
				return builder.String(), nil
			}
			builder.WriteString(text)

		case err, ok := <-r.ErrorChan:
			if ok && err != nil {
				// 发生错误
				return builder.String(), err
			}
			// 错误Channel已关闭且没有错误
			if len(r.TextChan) == 0 {
				return builder.String(), nil
			}
		}
	}
}

// ForEach 迭代处理每个文本块
// fn返回error时会停止迭代并返回该错误
//
// 使用示例:
//   err := resp.ForEach(func(text string) error {
//       fmt.Print(text)
//       return nil
//   })
func (r *StreamResponse) ForEach(fn func(text string) error) error {
	for {
		select {
		case text, ok := <-r.TextChan:
			if !ok {
				// Channel已关闭
				return nil
			}

			// 调用处理函数
			if err := fn(text); err != nil {
				// 用户函数返回错误，取消并退出
				if r.Cancel != nil {
					r.Cancel()
				}
				return err
			}

		case err, ok := <-r.ErrorChan:
			if ok && err != nil {
				// 发生错误
				return err
			}
			// 错误Channel已关闭且没有错误
			if len(r.TextChan) == 0 {
				return nil
			}
		}
	}
}

// CollectWithCallback 收集文本并在每次接收时调用回调
// 适合需要同时收集完整文本和实时处理的场景
//
// 使用示例:
//   fullText, err := resp.CollectWithCallback(func(accumulated, delta string) {
//       // accumulated: 到目前为止的完整文本
//       // delta: 本次新增的文本
//       fmt.Printf("新增: %s, 总长度: %d\n", delta, len(accumulated))
//   })
func (r *StreamResponse) CollectWithCallback(callback func(accumulated, delta string)) (string, error) {
	var builder strings.Builder

	for {
		select {
		case text, ok := <-r.TextChan:
			if !ok {
				// Channel已关闭
				return builder.String(), nil
			}

			builder.WriteString(text)
			if callback != nil {
				callback(builder.String(), text)
			}

		case err, ok := <-r.ErrorChan:
			if ok && err != nil {
				return builder.String(), err
			}
			if len(r.TextChan) == 0 {
				return builder.String(), nil
			}
		}
	}
}

// Close 关闭流式响应
// 调用Cancel函数以释放资源
func (r *StreamResponse) Close() {
	if r.Cancel != nil {
		r.Cancel()
	}
}

// StreamBuffer 流式缓冲区
// 用于在生产者和消费者之间提供缓冲
type StreamBuffer struct {
	textChan chan string
	errChan  chan error
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewStreamBuffer 创建流式缓冲区
func NewStreamBuffer(ctx context.Context, bufferSize int) *StreamBuffer {
	if bufferSize <= 0 {
		bufferSize = 10
	}

	ctx, cancel := context.WithCancel(ctx)

	return &StreamBuffer{
		textChan: make(chan string, bufferSize),
		errChan:  make(chan error, 1),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Write 写入文本到缓冲区
func (b *StreamBuffer) Write(text string) error {
	select {
	case <-b.ctx.Done():
		return b.ctx.Err()
	case b.textChan <- text:
		return nil
	}
}

// Error 写入错误并关闭
func (b *StreamBuffer) Error(err error) {
	select {
	case b.errChan <- err:
	default:
		// 错误Channel已满，忽略
	}
	b.Close()
}

// Close 关闭缓冲区
func (b *StreamBuffer) Close() {
	close(b.textChan)
	close(b.errChan)
}

// Channels 返回Channel对
func (b *StreamBuffer) Channels() (<-chan string, <-chan error) {
	return b.textChan, b.errChan
}

// Response 转换为StreamResponse
func (b *StreamBuffer) Response() *StreamResponse {
	return NewStreamResponseWithContext(b.ctx, b.textChan, b.errChan, b.cancel)
}


// MergeChannels 合并多个流式响应
// 将多个文本流合并为一个，按顺序输出
func MergeChannels(ctx context.Context, responses ...*StreamResponse) *StreamResponse {
	buffer := NewStreamBuffer(ctx, 20)

	go func() {
		defer buffer.Close()

		for _, resp := range responses {
			for {
				select {
				case <-ctx.Done():
					buffer.Error(ctx.Err())
					return

				case text, ok := <-resp.TextChan:
					if !ok {
						// 当前响应结束，继续下一个
						goto nextResponse
					}
					if err := buffer.Write(text); err != nil {
						buffer.Error(err)
						return
					}

				case err, ok := <-resp.ErrorChan:
					if ok && err != nil {
						buffer.Error(err)
						return
					}
				}
			}
		nextResponse:
		}
	}()

	return buffer.Response()
}

// ValidateStreamResponse 验证流式响应的有效性
func ValidateStreamResponse(resp *StreamResponse) error {
	if resp == nil {
		return fmt.Errorf("StreamResponse不能为nil")
	}
	if resp.TextChan == nil {
		return fmt.Errorf("TextChan不能为nil")
	}
	if resp.ErrorChan == nil {
		return fmt.Errorf("ErrorChan不能为nil")
	}
	return nil
}
