package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// StreamHandler 流式响应处理器
// 负责解析OpenAI SSE格式的流式响应
type StreamHandler struct {
	reader  io.Reader     // 原始响应流
	scanner *bufio.Scanner // 行扫描器
}

// NewStreamHandler 创建流式处理器
// reader: HTTP响应的Body
func NewStreamHandler(reader io.Reader) *StreamHandler {
	scanner := bufio.NewScanner(reader)
	// 设置较大的缓冲区，避免行过长导致扫描失败
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	return &StreamHandler{
		reader:  reader,
		scanner: scanner,
	}
}

// ReadStream 读取SSE流并解析为StreamChunk
// 返回Channel供消费者读取
//
// 使用示例:
//   chunkChan, errChan := handler.ReadStream(ctx)
//   for chunk := range chunkChan {
//       // 处理chunk
//   }
//   if err := <-errChan; err != nil {
//       // 处理错误
//   }
func (h *StreamHandler) ReadStream(ctx context.Context) (<-chan StreamChunk, <-chan error) {
	chunkChan := make(chan StreamChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
				// 继续读取
			}

			// 扫描下一行
			if !h.scanner.Scan() {
				// 检查是否有扫描错误
				if err := h.scanner.Err(); err != nil {
					errChan <- fmt.Errorf("扫描SSE流失败: %w", err)
					return
				}
				// 流结束
				return
			}

			line := h.scanner.Text()

			// 跳过空行
			if line == "" {
				continue
			}

			// 解析SSE行
			chunk, done, err := h.parseSSELine(line)
			if err != nil {
				errChan <- fmt.Errorf("解析SSE行失败: %w", err)
				return
			}

			// 检查是否收到结束信号
			if done {
				return
			}

			// 发送解析后的chunk
			if chunk != nil {
				select {
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				case chunkChan <- *chunk:
					// 成功发送
				}
			}
		}
	}()

	return chunkChan, errChan
}

// parseSSELine 解析单行SSE数据
// SSE格式: "data: {json}\n"
// 终止信号: "data: [DONE]\n"
//
// 返回值:
//   - chunk: 解析后的StreamChunk，如果不是有效的data行则为nil
//   - done: 是否收到[DONE]终止信号
//   - error: 解析错误
func (h *StreamHandler) parseSSELine(line string) (*StreamChunk, bool, error) {
	// 跳过非data行（如注释行）
	if !strings.HasPrefix(line, "data:") {
		return nil, false, nil
	}

	// 提取data内容
	data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

	// 检查结束信号
	if data == "[DONE]" {
		return nil, true, nil
	}

	// 跳过空data
	if data == "" {
		return nil, false, nil
	}

	// 解析JSON
	var chunk StreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil, false, fmt.Errorf("解析JSON失败: %w, 原始数据: %s", err, data)
	}

	return &chunk, false, nil
}

// ExtractContent 从StreamChunk中提取文本内容
// 这是一个辅助函数，用于从chunk中提取实际的文本内容
func ExtractContent(chunk StreamChunk) string {
	// 检查是否有选择项
	if len(chunk.Choices) == 0 {
		return ""
	}

	// 获取第一个选择的delta内容
	return chunk.Choices[0].Delta.Content
}

// IsFinished 检查流是否已完成
// 检查finish_reason是否非空
func IsFinished(chunk StreamChunk) bool {
	if len(chunk.Choices) == 0 {
		return false
	}

	return chunk.Choices[0].FinishReason != ""
}
