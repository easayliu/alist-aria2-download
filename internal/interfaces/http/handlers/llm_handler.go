package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	httputil "github.com/easayliu/alist-aria2-download/pkg/utils/http"
	"github.com/gin-gonic/gin"
)

// LLMHandler LLM HTTP处理器
type LLMHandler struct {
	container *services.ServiceContainer
}

// NewLLMHandler 创建LLM处理器
func NewLLMHandler(container *services.ServiceContainer) *LLMHandler {
	return &LLMHandler{
		container: container,
	}
}

// ==================== 请求/响应结构体 ====================

// GenerateRequest 通用LLM生成请求
type GenerateRequest struct {
	Prompt      string            `json:"prompt" binding:"required"`
	Model       string            `json:"model,omitempty"`
	Temperature float32           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Options     map[string]string `json:"options,omitempty"`
}

// GenerateResponse 通用LLM生成响应
type GenerateResponse struct {
	Text     string `json:"text"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// LLMRenameRequest LLM文件重命名请求
type LLMRenameRequest struct {
	FilePath string `json:"file_path" binding:"required"`
	Strategy string `json:"strategy,omitempty"` // tmdb_first, llm_first, llm_only, tmdb_only, compare
	UserHint string `json:"user_hint,omitempty"`
}

// BatchLLMRenameRequest 批量LLM重命名请求
type BatchLLMRenameRequest struct {
	FilePaths []string `json:"file_paths" binding:"required"`
	Strategy  string   `json:"strategy,omitempty"`
}

// BatchLLMRenameResponse 批量LLM重命名响应
type BatchLLMRenameResponse struct {
	Results []*contracts.FileRenameResponse `json:"results"`
	Total   int                             `json:"total"`
	Success int                             `json:"success"`
	Failed  int                             `json:"failed"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// ==================== HTTP端点实现 ====================

// Generate 处理通用生成请求
// @Summary LLM生成文本
// @Description 使用LLM生成文本内容
// @Tags LLM
// @Accept json
// @Produce json
// @Param request body GenerateRequest true "生成请求"
// @Success 200 {object} GenerateResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/llm/generate [post]
func (h *LLMHandler) Generate(c *gin.Context) {
	ctx := context.Background()
	var req GenerateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "无效的请求参数: "+err.Error())
		return
	}

	// 获取LLM服务
	llmService := h.container.GetLLMService()

	// 检查LLM是否启用
	if !llmService.IsEnabled() {
		httputil.ErrorWithStatus(c, http.StatusServiceUnavailable, 503, "LLM功能未启用，请在配置文件中配置LLM Provider")
		return
	}

	// 构建选项
	var opts []contracts.LLMOption
	if req.Model != "" {
		opts = append(opts, contracts.WithLLMModel(req.Model))
	}
	if req.Temperature > 0 {
		opts = append(opts, contracts.WithLLMTemperature(req.Temperature))
	}
	if req.MaxTokens > 0 {
		opts = append(opts, contracts.WithLLMMaxTokens(req.MaxTokens))
	}

	// 调用LLM生成
	text, err := llmService.GenerateText(ctx, req.Prompt, opts...)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "LLM生成失败: "+err.Error())
		return
	}

	// 返回响应
	httputil.Success(c, GenerateResponse{
		Text:     text,
		Provider: llmService.GetProviderName(),
		Model:    req.Model,
	})
}

// Stream 处理流式生成请求 (SSE)
// @Summary LLM流式生成文本
// @Description 使用Server-Sent Events流式返回LLM生成的文本
// @Tags LLM
// @Accept json
// @Produce text/event-stream
// @Param prompt query string true "生成提示词"
// @Param model query string false "模型名称"
// @Success 200 {string} string "text/event-stream"
// @Failure 400 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/llm/stream [get]
func (h *LLMHandler) Stream(c *gin.Context) {
	prompt := c.Query("prompt")
	if prompt == "" {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "缺少必需参数: prompt")
		return
	}

	model := c.Query("model")

	// 获取LLM服务
	llmService := h.container.GetLLMService()

	// 检查LLM是否启用
	if !llmService.IsEnabled() {
		httputil.ErrorWithStatus(c, http.StatusServiceUnavailable, 503, "LLM功能未启用")
		return
	}

	// 设置SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// 构建选项
	var opts []contracts.LLMOption
	if model != "" {
		opts = append(opts, contracts.WithLLMModel(model))
	}

	// 获取流式响应
	ctx := c.Request.Context()
	textChan, errChan := llmService.GenerateTextStream(ctx, prompt, opts...)

	// 持续推送
	c.Stream(func(w io.Writer) bool {
		select {
		case text, ok := <-textChan:
			if !ok {
				// 流结束
				c.SSEvent("message", map[string]interface{}{
					"text": "",
					"done": true,
				})
				return false
			}

			c.SSEvent("message", map[string]interface{}{
				"text": text,
				"done": false,
			})
			return true

		case err := <-errChan:
			c.SSEvent("error", map[string]interface{}{
				"error": err.Error(),
			})
			return false

		case <-ctx.Done():
			return false
		}
	})
}

// RenameWithLLM 处理文件重命名(统一使用批量TMDB模式)
// @Summary 文件重命名
// @Description 使用TMDB推断文件名
// @Tags LLM
// @Accept json
// @Produce json
// @Param request body LLMRenameRequest true "重命名请求"
// @Success 200 {object} contracts.FileRenameResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/files/rename-with-llm [post]
func (h *LLMHandler) RenameWithLLM(c *gin.Context) {
	ctx := context.Background()
	var req LLMRenameRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "无效的请求参数: "+err.Error())
		return
	}

	// 获取服务
	fileService := h.container.GetFileService()

	// 统一使用批量TMDB模式(即使只有单个文件)
	suggestionsMap, _, err := fileService.GetBatchRenameSuggestionsWithLLM(ctx, []string{req.FilePath})
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "重命名失败: "+err.Error())
		return
	}

	// 获取结果
	suggestions, found := suggestionsMap[req.FilePath]
	if !found || len(suggestions) == 0 {
		httputil.ErrorWithStatus(c, http.StatusNotFound, 404, "未找到匹配的重命名建议")
		return
	}

	// 返回第一个建议
	httputil.Success(c, suggestions[0])
}

// BatchRenameWithLLM 处理批量重命名(统一使用TMDB批量模式)
// @Summary 批量重命名
// @Description 批量使用TMDB推断文件名
// @Tags LLM
// @Accept json
// @Produce json
// @Param request body BatchLLMRenameRequest true "批量重命名请求"
// @Success 200 {object} BatchLLMRenameResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/files/batch-rename-with-llm [post]
func (h *LLMHandler) BatchRenameWithLLM(c *gin.Context) {
	ctx := context.Background()
	var req BatchLLMRenameRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "无效的请求参数: "+err.Error())
		return
	}

	if len(req.FilePaths) == 0 {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "文件路径列表不能为空")
		return
	}

	// 获取服务
	fileService := h.container.GetFileService()

	// 统一使用批量TMDB模式
	suggestionsMap, _, err := fileService.GetBatchRenameSuggestionsWithLLM(ctx, req.FilePaths)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "批量重命名失败: "+err.Error())
		return
	}

	// 转换为results数组
	results := make([]*contracts.RenameSuggestion, 0, len(req.FilePaths))
	for _, path := range req.FilePaths {
		if suggestions, found := suggestionsMap[path]; found && len(suggestions) > 0 {
			results = append(results, &suggestions[0])
		}
	}

	// 统计结果
	total := len(results)
	success := 0
	failed := 0
	for _, r := range results {
		if r.NewName != "" && r.Confidence > 0 {
			success++
		} else {
			failed++
		}
	}

	httputil.Success(c, gin.H{
		"results": results,
		"total":   total,
		"success": success,
		"failed":  failed,
	})
}

// ==================== 辅助方法 ====================

// ==================== 流式重命名端点（带进度回调）====================

// StreamRenameRequest 流式重命名请求
type StreamRenameRequest struct {
	FilePath string `json:"file_path" binding:"required"`
	UserHint string `json:"user_hint,omitempty"`
}

// StreamRename 流式重命名（SSE）
// @Summary 流式LLM文件重命名
// @Description 使用SSE流式返回LLM推断过程
// @Tags LLM
// @Accept json
// @Produce text/event-stream
// @Param request body StreamRenameRequest true "流式重命名请求"
// @Success 200 {string} string "text/event-stream"
// @Failure 400 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/files/rename-stream [post]
func (h *LLMHandler) StreamRename(c *gin.Context) {
	var req StreamRenameRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "无效的请求参数: "+err.Error())
		return
	}

	// 获取服务
	llmService := h.container.GetLLMService()

	// 检查LLM是否启用
	if !llmService.IsEnabled() {
		httputil.ErrorWithStatus(c, http.StatusServiceUnavailable, 503, "LLM功能未启用")
		return
	}

	// 设置SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ctx := c.Request.Context()

	// 构建提示词
	prompt := fmt.Sprintf("请根据文件名推断影视作品的标准名称: %s", req.FilePath)
	if req.UserHint != "" {
		prompt += fmt.Sprintf("\n用户提示: %s", req.UserHint)
	}

	// 获取流式响应
	textChan, errChan := llmService.GenerateTextStream(ctx, prompt)

	// 节流控制
	lastUpdate := time.Now()
	partialText := ""

	c.Stream(func(w io.Writer) bool {
		select {
		case text, ok := <-textChan:
			if !ok {
				// 流结束，发送最终结果
				c.SSEvent("complete", map[string]interface{}{
					"text": partialText,
					"done": true,
				})
				return false
			}

			partialText += text

			// 节流：每500ms更新一次
			if time.Since(lastUpdate) >= 500*time.Millisecond {
				lastUpdate = time.Now()
				c.SSEvent("progress", map[string]interface{}{
					"text": partialText,
					"done": false,
				})
			}
			return true

		case err := <-errChan:
			c.SSEvent("error", map[string]interface{}{
				"error": err.Error(),
			})
			return false

		case <-ctx.Done():
			return false
		}
	})
}
