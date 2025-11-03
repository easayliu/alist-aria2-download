package llm

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// StreamProcessor 流式处理器
// 提供流式文本的累积、节流和回调处理能力
type StreamProcessor struct {
	bufferSize    int           // 缓冲区大小
	updateHandler UpdateHandler // 更新回调函数
}

// UpdateHandler 更新回调函数
// accumulated: 累积的完整文本
// delta: 本次新增的文本
type UpdateHandler func(accumulated string, delta string) error

// NewStreamProcessor 创建流式处理器
func NewStreamProcessor(handler UpdateHandler) *StreamProcessor {
	return &StreamProcessor{
		bufferSize:    10,
		updateHandler: handler,
	}
}

// NewStreamProcessorWithBuffer 创建带自定义缓冲区的流式处理器
func NewStreamProcessorWithBuffer(handler UpdateHandler, bufferSize int) *StreamProcessor {
	if bufferSize <= 0 {
		bufferSize = 10
	}
	return &StreamProcessor{
		bufferSize:    bufferSize,
		updateHandler: handler,
	}
}

// Process 处理流式响应
// 自动累积文本，实时触发回调
//
// 使用示例:
//   processor := NewStreamProcessor(func(accumulated, delta string) error {
//       fmt.Printf("累积文本长度: %d, 新增: %s\n", len(accumulated), delta)
//       return nil
//   })
//   fullText, err := processor.Process(ctx, textChan, errChan)
func (p *StreamProcessor) Process(ctx context.Context, textChan <-chan string, errChan <-chan error) (string, error) {
	var builder strings.Builder

	for {
		select {
		case <-ctx.Done():
			return builder.String(), ctx.Err()

		case text, ok := <-textChan:
			if !ok {
				// 流已结束
				return builder.String(), nil
			}

			// 累积文本
			builder.WriteString(text)

			// 触发回调
			if p.updateHandler != nil {
				if err := p.updateHandler(builder.String(), text); err != nil {
					return builder.String(), fmt.Errorf("更新回调失败: %w", err)
				}
			}

		case err, ok := <-errChan:
			if ok && err != nil {
				return builder.String(), err
			}
			// 错误Channel关闭但没有错误，继续读取文本
			if len(textChan) == 0 {
				return builder.String(), nil
			}
		}
	}
}

// ProcessWithThrottle 带节流的处理
// minInterval: 最小回调间隔（避免频繁更新）
//
// 适用场景:
//   - Telegram消息更新（推荐500ms-1s）
//   - UI界面更新（推荐100ms-300ms）
//
// 使用示例:
//   processor := NewStreamProcessor(func(accumulated, delta string) error {
//       // 更新Telegram消息
//       return bot.EditMessage(chatID, messageID, accumulated)
//   })
//   fullText, err := processor.ProcessWithThrottle(ctx, textChan, errChan, 500*time.Millisecond)
func (p *StreamProcessor) ProcessWithThrottle(ctx context.Context, textChan <-chan string, errChan <-chan error, minInterval time.Duration) (string, error) {
	var builder strings.Builder
	var lastUpdate time.Time
	var pendingUpdate bool

	// 启动定时器用于触发延迟的更新
	ticker := time.NewTicker(minInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context取消，如果有待处理的更新则执行最后一次
			if pendingUpdate && p.updateHandler != nil {
				_ = p.updateHandler(builder.String(), "")
			}
			return builder.String(), ctx.Err()

		case text, ok := <-textChan:
			if !ok {
				// 流已结束，触发最后一次更新
				if pendingUpdate && p.updateHandler != nil {
					if err := p.updateHandler(builder.String(), ""); err != nil {
						return builder.String(), fmt.Errorf("最终更新回调失败: %w", err)
					}
				}
				return builder.String(), nil
			}

			// 累积文本
			builder.WriteString(text)
			pendingUpdate = true

			// 检查是否可以立即更新
			now := time.Now()
			if now.Sub(lastUpdate) >= minInterval {
				if p.updateHandler != nil {
					if err := p.updateHandler(builder.String(), text); err != nil {
						return builder.String(), fmt.Errorf("更新回调失败: %w", err)
					}
				}
				lastUpdate = now
				pendingUpdate = false
			}

		case <-ticker.C:
			// 定时器触发，检查是否有待处理的更新
			if pendingUpdate && p.updateHandler != nil {
				if err := p.updateHandler(builder.String(), ""); err != nil {
					return builder.String(), fmt.Errorf("节流更新回调失败: %w", err)
				}
				lastUpdate = time.Now()
				pendingUpdate = false
			}

		case err, ok := <-errChan:
			if ok && err != nil {
				// 发生错误，触发最后一次更新
				if pendingUpdate && p.updateHandler != nil {
					_ = p.updateHandler(builder.String(), "")
				}
				return builder.String(), err
			}
		}
	}
}

// ProcessBatch 批量处理流式响应
// batchSize: 累积多少个文本块后触发一次回调
//
// 适用场景:
//   - 减少回调次数
//   - 批量写入数据库
//
// 使用示例:
//   processor := NewStreamProcessor(func(accumulated, delta string) error {
//       // 每累积10个块写一次数据库
//       return db.SaveProgress(accumulated)
//   })
//   fullText, err := processor.ProcessBatch(ctx, textChan, errChan, 10)
func (p *StreamProcessor) ProcessBatch(ctx context.Context, textChan <-chan string, errChan <-chan error, batchSize int) (string, error) {
	if batchSize <= 0 {
		batchSize = 5
	}

	var builder strings.Builder
	var batchBuilder strings.Builder
	count := 0

	for {
		select {
		case <-ctx.Done():
			// 处理剩余批次
			if batchBuilder.Len() > 0 && p.updateHandler != nil {
				_ = p.updateHandler(builder.String(), batchBuilder.String())
			}
			return builder.String(), ctx.Err()

		case text, ok := <-textChan:
			if !ok {
				// 流结束，处理剩余批次
				if batchBuilder.Len() > 0 && p.updateHandler != nil {
					if err := p.updateHandler(builder.String(), batchBuilder.String()); err != nil {
						return builder.String(), fmt.Errorf("最终批次回调失败: %w", err)
					}
				}
				return builder.String(), nil
			}

			// 累积文本
			builder.WriteString(text)
			batchBuilder.WriteString(text)
			count++

			// 检查是否达到批次大小
			if count >= batchSize {
				if p.updateHandler != nil {
					if err := p.updateHandler(builder.String(), batchBuilder.String()); err != nil {
						return builder.String(), fmt.Errorf("批次回调失败: %w", err)
					}
				}
				// 重置批次
				batchBuilder.Reset()
				count = 0
			}

		case err, ok := <-errChan:
			if ok && err != nil {
				// 处理剩余批次
				if batchBuilder.Len() > 0 && p.updateHandler != nil {
					_ = p.updateHandler(builder.String(), batchBuilder.String())
				}
				return builder.String(), err
			}
		}
	}
}

// SimpleProcess 简化的处理方法，不使用回调
// 直接返回完整文本
func SimpleProcess(ctx context.Context, textChan <-chan string, errChan <-chan error) (string, error) {
	processor := NewStreamProcessor(nil)
	return processor.Process(ctx, textChan, errChan)
}

// ProcessWithProgress 带进度通知的处理
// progressChan: 进度Channel，发送累积的文本长度
//
// 使用示例:
//   progressChan := make(chan int, 1)
//   go func() {
//       for progress := range progressChan {
//           fmt.Printf("进度: %d 字符\n", progress)
//       }
//   }()
//   fullText, err := ProcessWithProgress(ctx, textChan, errChan, progressChan)
func ProcessWithProgress(ctx context.Context, textChan <-chan string, errChan <-chan error, progressChan chan<- int) (string, error) {
	defer close(progressChan)

	var builder strings.Builder

	for {
		select {
		case <-ctx.Done():
			return builder.String(), ctx.Err()

		case text, ok := <-textChan:
			if !ok {
				return builder.String(), nil
			}

			builder.WriteString(text)

			// 发送进度
			select {
			case progressChan <- builder.Len():
			default:
				// Channel已满，跳过本次进度更新
			}

		case err, ok := <-errChan:
			if ok && err != nil {
				return builder.String(), err
			}
		}
	}
}

// ThrottledUpdateHandler 创建节流的更新处理器
// 这是一个高阶函数，用于包装已有的UpdateHandler
//
// 使用示例:
//   originalHandler := func(accumulated, delta string) error {
//       return bot.EditMessage(chatID, messageID, accumulated)
//   }
//   throttledHandler := ThrottledUpdateHandler(originalHandler, 500*time.Millisecond)
//   processor := NewStreamProcessor(throttledHandler)
func ThrottledUpdateHandler(handler UpdateHandler, minInterval time.Duration) UpdateHandler {
	var lastUpdate time.Time
	var mutex sync.Mutex

	return func(accumulated, delta string) error {
		mutex.Lock()
		defer mutex.Unlock()

		now := time.Now()
		if now.Sub(lastUpdate) < minInterval {
			// 距离上次更新时间太短，跳过
			return nil
		}

		lastUpdate = now
		return handler(accumulated, delta)
	}
}
