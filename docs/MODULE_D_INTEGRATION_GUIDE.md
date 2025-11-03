# 模块D集成指南：LLM文件重命名功能

## 概述

本文档说明如何将模块D（LLM文件重命名场景集成）整合到现有系统中。

## 前置依赖

### Agent-C的LLM服务

模块D依赖Agent-C提供的LLM服务接口，确保以下接口已实现：

```go
// internal/application/contracts/llm_contract.go
type LLMService interface {
    GenerateStructured(ctx context.Context, prompt string, schema interface{}) (string, error)
    GenerateTextStream(ctx context.Context, prompt string) (<-chan string, <-chan error)
    // ... 其他方法
}
```

如果Agent-C尚未完成，可以先创建Mock实现进行测试：

```go
// internal/infrastructure/llm/mock_service.go
type MockLLMService struct{}

func (m *MockLLMService) GenerateStructured(ctx context.Context, prompt string, schema interface{}) (string, error) {
    // 返回模拟的JSON响应
    return `{
        "media_type": "tv",
        "title": "Test Show",
        "year": 2024,
        "season": 1,
        "episode": 1,
        "confidence": 0.9
    }`, nil
}

func (m *MockLLMService) GenerateTextStream(ctx context.Context, prompt string) (<-chan string, <-chan error) {
    textChan := make(chan string, 1)
    errChan := make(chan error, 1)

    go func() {
        textChan <- `{"media_type": "tv"...}`
        close(textChan)
        close(errChan)
    }()

    return textChan, errChan
}
```

## 集成步骤

### 1. 初始化LLM服务

在服务容器中添加LLM服务的初始化：

```go
// internal/application/services/service_container.go

type ServiceContainer struct {
    // ... 现有字段
    llmService contracts.LLMService
}

func (sc *ServiceContainer) InitializeLLMService() error {
    // 检查配置
    if sc.config.LLM.APIKey == "" {
        logger.Info("LLM服务未配置，跳过初始化")
        return nil
    }

    // 创建LLM服务实例（根据提供商）
    switch sc.config.LLM.Provider {
    case "openai":
        sc.llmService = openai.NewService(sc.config.LLM)
    case "anthropic":
        sc.llmService = anthropic.NewService(sc.config.LLM)
    case "mock":
        sc.llmService = &MockLLMService{}
    default:
        return fmt.Errorf("不支持的LLM提供商: %s", sc.config.LLM.Provider)
    }

    // 将LLM服务设置到FileService
    if fileService, ok := sc.fileService.(*file.AppFileService); ok {
        fileService.SetLLMService(sc.llmService)
        logger.Info("LLM服务已注入到FileService")
    }

    return nil
}
```

### 2. 配置文件更新

在配置文件中添加LLM相关配置：

```yaml
# config.yaml
llm:
  provider: "openai"        # openai, anthropic, mock
  api_key: "sk-xxx"         # API密钥
  model: "gpt-4"            # 模型名称
  base_url: ""              # 可选，自定义API地址
  max_tokens: 2000          # 最大生成token数
  temperature: 0.7          # 温度参数
  timeout: 30               # 超时时间（秒）
  retry_count: 3            # 重试次数
  enable_stream: true       # 启用流式输出
```

### 3. HTTP API接口

在HTTP层添加新的端点：

```go
// internal/interfaces/http/handlers/file_handler.go

// RenameWithLLM 使用LLM推断重命名
func (h *FileHandler) RenameWithLLM(c *fiber.Ctx) error {
    var req contracts.FileRenameRequest
    if err := c.BodyParser(&req); err != nil {
        return fiber.NewError(fiber.StatusBadRequest, "无效的请求")
    }

    resp, err := h.fileService.SuggestFileNameWithLLM(c.Context(), req)
    if err != nil {
        return fiber.NewError(fiber.StatusInternalServerError, err.Error())
    }

    return c.JSON(resp)
}

// RenameWithHybrid 使用混合策略推断重命名
func (h *FileHandler) RenameWithHybrid(c *fiber.Ctx) error {
    var req struct {
        contracts.FileRenameRequest
        Strategy string `json:"strategy"` // "tmdb_first", "llm_first", etc.
    }

    if err := c.BodyParser(&req); err != nil {
        return fiber.NewError(fiber.StatusBadRequest, "无效的请求")
    }

    // 解析策略
    strategy := parseStrategy(req.Strategy)

    resp, err := h.fileService.SuggestFileNameHybrid(c.Context(), req.FileRenameRequest, strategy)
    if err != nil {
        return fiber.NewError(fiber.StatusInternalServerError, err.Error())
    }

    return c.JSON(resp)
}

// RenameCompare 比较模式
func (h *FileHandler) RenameCompare(c *fiber.Ctx) error {
    var req contracts.FileRenameRequest
    if err := c.BodyParser(&req); err != nil {
        return fiber.NewError(fiber.StatusBadRequest, "无效的请求")
    }

    responses, err := h.fileService.SuggestFileNameWithCompare(c.Context(), req)
    if err != nil {
        return fiber.NewError(fiber.StatusInternalServerError, err.Error())
    }

    return c.JSON(responses)
}

func parseStrategy(strategy string) contracts.HybridStrategy {
    switch strategy {
    case "tmdb_first":
        return contracts.TMDBFirst
    case "llm_first":
        return contracts.LLMFirst
    case "tmdb_only":
        return contracts.TMDBOnly
    case "llm_only":
        return contracts.LLMOnly
    case "compare":
        return contracts.Compare
    default:
        return contracts.TMDBFirst
    }
}
```

### 4. 路由注册

```go
// internal/interfaces/http/routes/routes.go

func SetupRoutes(app *fiber.App, handlers *Handlers) {
    // ... 现有路由

    // LLM增强的重命名路由（新增）
    api.Post("/files/rename/llm", handlers.FileHandler.RenameWithLLM)
    api.Post("/files/rename/hybrid", handlers.FileHandler.RenameWithHybrid)
    api.Post("/files/rename/compare", handlers.FileHandler.RenameCompare)
    api.Post("/files/batch-rename/llm", handlers.FileHandler.BatchRenameWithLLM)
}
```

### 5. Telegram集成

在Telegram Bot中添加命令：

```go
// internal/interfaces/telegram/commands/rename_commands.go

// HandleRenameWithLLM 处理/rename_llm命令
func (h *TelegramHandler) HandleRenameWithLLM(update tgbotapi.Update) error {
    // 获取文件路径
    filePath := extractFilePath(update.Message.Text)

    // 发送处理中消息
    msg := tgbotapi.NewMessage(update.Message.Chat.ID, "🤖 正在使用LLM分析文件名...")
    sentMsg, _ := h.bot.Send(msg)

    // 调用LLM推断
    resp, err := h.fileService.SuggestFileNameWithLLM(context.Background(), contracts.FileRenameRequest{
        OriginalPath: filePath,
    })

    if err != nil {
        h.bot.Send(tgbotapi.NewEditMessageText(
            update.Message.Chat.ID,
            sentMsg.MessageID,
            fmt.Sprintf("❌ 推断失败: %s", err.Error()),
        ))
        return err
    }

    // 格式化结果
    result := fmt.Sprintf(
        "✅ LLM推断结果\n\n"+
        "原文件名: %s\n"+
        "建议名称: %s\n\n"+
        "媒体信息:\n"+
        "类型: %s\n"+
        "标题: %s\n"+
        "年份: %d\n"+
        "置信度: %.2f\n\n"+
        "是否执行重命名？",
        resp.OriginalName,
        resp.SuggestedName,
        resp.MediaInfo.Type,
        resp.MediaInfo.Title,
        resp.MediaInfo.Year,
        resp.Confidence,
    )

    // 发送结果和确认按钮
    h.bot.Send(tgbotapi.NewEditMessageText(
        update.Message.Chat.ID,
        sentMsg.MessageID,
        result,
    ))

    return nil
}

// HandleRenameCompare 处理/rename_compare命令（比较模式）
func (h *TelegramHandler) HandleRenameCompare(update tgbotapi.Update) error {
    filePath := extractFilePath(update.Message.Text)

    msg := tgbotapi.NewMessage(update.Message.Chat.ID, "🔍 正在比较TMDB和LLM结果...")
    sentMsg, _ := h.bot.Send(msg)

    responses, err := h.fileService.SuggestFileNameWithCompare(context.Background(), contracts.FileRenameRequest{
        OriginalPath: filePath,
    })

    if err != nil {
        h.bot.Send(tgbotapi.NewEditMessageText(
            update.Message.Chat.ID,
            sentMsg.MessageID,
            fmt.Sprintf("❌ 比较失败: %s", err.Error()),
        ))
        return err
    }

    // 构建比较结果
    var result strings.Builder
    result.WriteString("📊 比较结果\n\n")

    for i, resp := range responses {
        result.WriteString(fmt.Sprintf(
            "选项%d [%s]:\n"+
            "建议: %s\n"+
            "置信度: %.2f\n\n",
            i+1,
            resp.Source,
            resp.SuggestedName,
            resp.Confidence,
        ))
    }

    result.WriteString("请选择要使用的选项：")

    // 发送结果
    h.bot.Send(tgbotapi.NewEditMessageText(
        update.Message.Chat.ID,
        sentMsg.MessageID,
        result.String(),
    ))

    return nil
}
```

### 6. 命令注册

```go
// internal/interfaces/telegram/telegram_handler.go

func (h *TelegramHandler) RegisterCommands() {
    // ... 现有命令

    // LLM相关命令（新增）
    h.commands["rename_llm"] = h.HandleRenameWithLLM
    h.commands["rename_compare"] = h.HandleRenameCompare
    h.commands["rename_hybrid"] = h.HandleRenameWithHybrid
}
```

## 使用示例

### HTTP API调用

#### 1. LLM推断

```bash
curl -X POST http://localhost:8080/api/files/rename/llm \
  -H "Content-Type: application/json" \
  -d '{
    "original_path": "/data/tvs/电视剧名.S01E01.mkv",
    "user_hint": "这是一部美剧"
  }'
```

响应：
```json
{
  "original_name": "电视剧名.S01E01.mkv",
  "suggested_name": "TV Show Title - S01E01.mkv",
  "confidence": 0.92,
  "source": "llm",
  "media_info": {
    "type": "tv",
    "title": "TV Show Title",
    "title_cn": "电视剧名",
    "year": 2020,
    "season": 1,
    "episode": 1
  }
}
```

#### 2. 混合策略

```bash
curl -X POST http://localhost:8080/api/files/rename/hybrid \
  -H "Content-Type: application/json" \
  -d '{
    "original_path": "/data/movies/The.Matrix.1999.mkv",
    "strategy": "tmdb_first"
  }'
```

#### 3. 比较模式

```bash
curl -X POST http://localhost:8080/api/files/rename/compare \
  -H "Content-Type: application/json" \
  -d '{
    "original_path": "/data/tvs/神秘博士.S01E01.mkv"
  }'
```

响应（多个选项）：
```json
[
  {
    "original_name": "神秘博士.S01E01.mkv",
    "suggested_name": "Doctor Who - S01E01.mkv",
    "confidence": 0.95,
    "source": "tmdb",
    "media_info": { ... }
  },
  {
    "original_name": "神秘博士.S01E01.mkv",
    "suggested_name": "神秘博士 - S01E01.mkv",
    "confidence": 0.88,
    "source": "llm",
    "media_info": { ... }
  }
]
```

### Telegram Bot命令

```
# LLM推断
/rename_llm /data/tvs/电视剧名.S01E01.mkv

# 混合推断（TMDB优先）
/rename_hybrid tmdb_first /data/movies/The.Matrix.1999.mkv

# 比较模式
/rename_compare /data/tvs/电视剧名.S01E01.mkv
```

## 错误处理

### 1. LLM服务未配置

```go
resp, err := fileService.SuggestFileNameWithLLM(ctx, req)
if err != nil {
    if err.Error() == "LLM服务未配置" {
        // 回退到纯TMDB模式
        return fileService.GetRenameSuggestions(ctx, req.OriginalPath)
    }
}
```

### 2. 置信度过低

```go
if resp.Confidence < 0.7 {
    // 使用比较模式让用户选择
    responses, _ := fileService.SuggestFileNameWithCompare(ctx, req)
    // 展示多个选项给用户
}
```

### 3. 超时处理

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

resp, err := fileService.SuggestFileNameWithLLM(ctx, req)
if err == context.DeadlineExceeded {
    // 超时，回退到TMDB
    return fileService.GetRenameSuggestions(ctx, req.OriginalPath)
}
```

## 性能优化

### 1. 批量处理

```go
// 使用协程并发处理
files := []string{"file1.mkv", "file2.mkv", "file3.mkv"}
responses, _ := fileService.BatchRenameWithLLM(ctx, files, contracts.TMDBFirst)
```

### 2. 缓存

在FileService中添加缓存层：

```go
type CachedFileService struct {
    fileService contracts.FileService
    cache       sync.Map // 线程安全的map
}

func (c *CachedFileService) SuggestFileNameWithLLM(ctx context.Context, req contracts.FileRenameRequest) (*contracts.FileRenameResponse, error) {
    // 检查缓存
    if cached, ok := c.cache.Load(req.OriginalPath); ok {
        return cached.(*contracts.FileRenameResponse), nil
    }

    // 调用实际服务
    resp, err := c.fileService.SuggestFileNameWithLLM(ctx, req)
    if err == nil {
        // 缓存结果
        c.cache.Store(req.OriginalPath, resp)
    }

    return resp, err
}
```

## 监控和日志

### 关键指标

```go
// 记录推断性能
start := time.Now()
resp, err := fileService.SuggestFileNameWithLLM(ctx, req)
duration := time.Since(start)

logger.Info("LLM推断完成",
    "duration_ms", duration.Milliseconds(),
    "source", resp.Source,
    "confidence", resp.Confidence)
```

### Prometheus指标

```go
var (
    llmRenameCounter = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "llm_rename_total",
            Help: "Total number of LLM rename requests",
        },
        []string{"source", "status"},
    )

    llmRenameDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "llm_rename_duration_seconds",
            Help: "Duration of LLM rename requests",
        },
        []string{"source"},
    )
)
```

## 测试

### 单元测试

```bash
cd internal/domain/services/filename
go test -v
```

### 集成测试

```bash
cd internal/application/services/file
go test -v -tags=integration
```

## 故障排查

### 问题检查清单

- [ ] LLM服务是否正确配置
- [ ] API密钥是否有效
- [ ] 网络连接是否正常
- [ ] FileService是否已注入LLM服务
- [ ] 日志中是否有错误信息

### 调试模式

```yaml
# config.yaml
logger:
  level: debug  # 启用详细日志

llm:
  timeout: 60   # 增加超时时间用于调试
```

## 迁移指南

如果已有使用旧重命名API的代码，迁移步骤：

### Before（旧API）
```go
suggestions, err := fileService.GetRenameSuggestions(ctx, filePath)
```

### After（新API，向后兼容）
```go
// 方式1：使用混合策略（推荐）
resp, err := fileService.SuggestFileNameHybrid(ctx,
    contracts.FileRenameRequest{OriginalPath: filePath},
    contracts.TMDBFirst)

// 方式2：继续使用旧API（如果LLM未配置，自动回退）
suggestions, err := fileService.GetRenameSuggestions(ctx, filePath)
```

## 下一步

- [ ] Agent-C完成LLM服务实现
- [ ] 添加更多LLM提供商支持（Claude, Gemini等）
- [ ] 实现prompt模板管理
- [ ] 添加用户反馈机制（改进推断质量）
- [ ] 支持自定义命名规则

## 联系方式

如有问题，请联系：
- Agent-D: 负责文件重命名模块
- Agent-C: 负责LLM服务基础设施
