# API First 架构重构迁移指南

## 🎯 重构目标

将现有的违反API First原则的架构重构为清晰的分层架构，消除重复代码，实现业务逻辑与表现层的完全解耦。

## 📋 重构前后对比

### 重构前的问题

1. **业务逻辑重复**：
   ```go
   // handlers/download.go - 重复的业务逻辑
   func CreateDownload(c *gin.Context) {
       cfg, err := config.LoadConfig()           // 重复配置加载
       aria2Client := aria2.NewClient(...)       // 重复客户端创建
       gid, err := aria2Client.AddURI(...)       // 业务逻辑在Handler层
   }

   // telegram/commands/download_commands.go - 同样的业务逻辑重复
   func (dc *DownloadCommands) HandleDownload(...) {
       download, err := dc.downloadService.CreateDownload(...)  // 不一致的调用
   }
   ```

2. **违反API First原则**：
   - Handler层承担业务职责
   - 不同客户端无法共享业务逻辑
   - 缺乏统一的业务接口契约

### 重构后的架构

```
┌─────────────────────────────────────────────────────────────┐
│                   API First 架构                             │
├─────────────────────────────────────────────────────────────┤
│  Interface Layer (协议转换层)                                │
│  ├── REST API Handler     ├── Telegram Handler              │
│  │   - 仅负责协议转换     │   - 仅负责协议转换               │
│  │   - 参数绑定/验证     │   - 消息格式转换                 │
│  │   - 响应格式化        │   - 错误处理                     │
├─────────────────────────────────────────────────────────────┤
│  Application Layer (应用服务层 - 业务流程编排)               │
│  ├── AppDownloadService  ├── AppTaskService                  │
│  │   - 下载业务流程     │   - 任务业务流程                 │
│  │   - 业务规则验证     │   - 调度逻辑编排                 │
│  │   - 服务编排         │   - 执行控制                     │
├─────────────────────────────────────────────────────────────┤
│  Domain Layer (领域层 - 核心业务逻辑)                        │
│  ├── Business Contracts  ├── Domain Services                │
│  │   - 统一接口契约     │   - 纯业务逻辑                   │
│  │   - 数据传输对象     │   - 领域规则                     │
├─────────────────────────────────────────────────────────────┤
│  Infrastructure Layer (基础设施层)                           │
│  ├── Aria2 Client       ├── AList Client                    │
│  │   - 外部系统集成     │   - 文件系统访问                 │
│  │   - 数据持久化       │   - 配置管理                     │
└─────────────────────────────────────────────────────────────┘
```

## 🔄 迁移步骤

### 第一步：创建业务契约层 ✅

```go
// internal/application/contracts/download_contract.go
type DownloadService interface {
    CreateDownload(ctx context.Context, req DownloadRequest) (*DownloadResponse, error)
    GetDownload(ctx context.Context, id string) (*DownloadResponse, error)
    ListDownloads(ctx context.Context, req DownloadListRequest) (*DownloadListResponse, error)
    // ... 其他方法
}

// 统一的数据传输对象
type DownloadRequest struct {
    URL          string                 `json:"url" validate:"required,url"`
    Filename     string                 `json:"filename,omitempty"`
    Directory    string                 `json:"directory,omitempty"`
    VideoOnly    bool                   `json:"video_only,omitempty"`
    AutoClassify bool                   `json:"auto_classify,omitempty"`
}
```

### 第二步：实现应用服务层 ✅

```go
// internal/application/services/app_download_service.go
type AppDownloadService struct {
    config      *config.Config
    aria2Client *aria2.Client
    fileService contracts.FileService
}

func (s *AppDownloadService) CreateDownload(ctx context.Context, req contracts.DownloadRequest) (*contracts.DownloadResponse, error) {
    // 1. 参数验证
    if err := s.validateDownloadRequest(req); err != nil {
        return nil, fmt.Errorf("invalid request: %w", err)
    }

    // 2. 应用业务规则
    if err := s.applyBusinessRules(&req); err != nil {
        return nil, fmt.Errorf("business rule violation: %w", err)
    }

    // 3. 执行业务逻辑 - 统一实现
    options := s.prepareDownloadOptions(req)
    gid, err := s.aria2Client.AddURI(req.URL, options)
    // ... 返回统一格式
}
```

### 第三步：重构Handler层 ✅

#### REST API Handler

```go
// internal/interfaces/api/rest/download_handler.go
type DownloadHandler struct {
    downloadService contracts.DownloadService  // 依赖接口，不依赖具体实现
}

func (h *DownloadHandler) CreateDownload(c *gin.Context) {
    var req contracts.DownloadRequest

    // 1. 协议转换 - 绑定请求参数
    if err := c.ShouldBindJSON(&req); err != nil {
        utils.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
        return
    }

    // 2. 调用业务服务 - 统一的业务逻辑
    response, err := h.downloadService.CreateDownload(c.Request.Context(), req)
    if err != nil {
        utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to create download: "+err.Error())
        return
    }

    // 3. 协议转换 - 返回响应
    utils.Success(c, response)
}
```

#### Telegram Handler

```go
// internal/interfaces/api/telegram/download_handler.go
type TelegramDownloadHandler struct {
    downloadService contracts.DownloadService  // 同样的业务服务接口
    messageUtils    types.MessageSender
}

func (h *TelegramDownloadHandler) HandleDownload(chatID int64, command string) {
    // 1. 协议转换 - 解析Telegram命令
    url := parseURLFromCommand(command)
    req := contracts.DownloadRequest{
        URL:          url,
        VideoOnly:    true,
        AutoClassify: true,
    }

    // 2. 调用业务服务 - 相同的业务逻辑
    response, err := h.downloadService.CreateDownload(context.Background(), req)
    if err != nil {
        h.messageUtils.SendMessage(chatID, "创建下载失败: "+err.Error())
        return
    }

    // 3. 协议转换 - 格式化Telegram消息
    message := h.formatDownloadResponse(response)
    h.messageUtils.SendMessageHTML(chatID, message)
}
```

### 第四步：实现依赖注入容器 ✅

```go
// internal/application/container/service_container.go
type ServiceContainer struct {
    downloadService contracts.DownloadService
    taskService     contracts.TaskService
    fileService     contracts.FileService
}

func (c *ServiceContainer) initServices() {
    // 按依赖关系初始化服务
    c.fileService = services.NewAppFileService(c.config, nil)
    c.downloadService = services.NewAppDownloadService(c.config, c.fileService)
    c.taskService = services.NewAppTaskService(c.config, c.taskRepo, c.schedulerService, c.downloadService, c.fileService)
}
```

## 🔧 实际迁移操作

### 1. 保持向后兼容

在迁移期间，保持旧版Handler的运行，逐步替换：

```go
// 旧版路由（保持运行）
v1.POST("/downloads", handlers.CreateDownload)

// 新版路由（逐步迁移）
v2.POST("/downloads", newDownloadHandler.CreateDownload)
```

### 2. 渐进式迁移

```bash
# 第一阶段：创建新架构（不影响现有功能）
- 创建 contracts 包
- 创建 application services
- 创建新的 interfaces 层

# 第二阶段：测试新架构
- 并行运行新旧系统
- 对比功能一致性
- 性能测试

# 第三阶段：切换流量
- 逐步将流量从旧Handler切换到新Handler
- 监控错误率和性能

# 第四阶段：清理旧代码
- 删除旧的Handler实现
- 删除重复的业务逻辑
- 更新文档
```

### 3. 关键文件迁移映射

| 旧文件 | 新文件 | 作用 |
|--------|--------|------|
| `handlers/download.go` | `interfaces/api/rest/download_handler.go` | REST API协议转换 |
| `telegram/commands/download_commands.go` | `interfaces/api/telegram/download_handler.go` | Telegram协议转换 |
| `services/download_service.go` | `application/services/app_download_service.go` | 统一业务逻辑 |
| - | `application/contracts/download_contract.go` | 业务接口契约 |
| - | `application/container/service_container.go` | 依赖注入容器 |

## 🧪 验证迁移成功

### 1. 功能验证

```bash
# 验证REST API
curl -X POST localhost:8080/api/v1/downloads \
  -H "Content-Type: application/json" \
  -d '{"url": "http://example.com/file.mp4", "auto_classify": true}'

# 验证Telegram Bot
/download http://example.com/file.mp4

# 验证业务逻辑一致性
- 两种方式创建的下载应该使用相同的分类逻辑
- 错误处理应该一致
- 配置应用应该一致
```

### 2. 架构验证

```go
// 确保Handler层不包含业务逻辑
func TestHandlerOnlyDoesProtocolConversion(t *testing.T) {
    handler := &DownloadHandler{downloadService: mockService}
    // Handler 应该只做参数绑定和响应格式化
    // 不应该包含 aria2Client.AddURI 等业务调用
}

// 确保业务逻辑可以被不同客户端重用
func TestBusinessLogicReusability(t *testing.T) {
    req := contracts.DownloadRequest{URL: "http://example.com/file.mp4"}
    
    // 相同的请求，从不同入口调用，应该产生相同结果
    restResult := restHandler.CreateDownload(restContext, req)
    telegramResult := telegramHandler.HandleDownload(chatID, "/download " + req.URL)
    
    assert.Equal(t, restResult.ID, telegramResult.ID)
}
```

## 🎉 重构收益

### 1. 代码复用提升

- **重复代码消除**：下载逻辑从2个地方减少到1个地方
- **一致性保证**：所有客户端使用相同的业务逻辑
- **维护成本降低**：修改业务逻辑只需要在一个地方进行

### 2. 架构清晰度提升

- **职责明确**：Handler层只负责协议转换
- **依赖清晰**：通过接口依赖，便于测试和替换
- **扩展性强**：添加新的客户端协议变得简单

### 3. API First实现

- **统一的业务契约**：所有客户端共享相同的业务接口
- **协议无关**：业务逻辑不依赖于特定的通信协议
- **测试友好**：可以独立测试业务逻辑和协议转换

## 🚀 下一步计划

1. **完成任务管理模块重构** - 应用相同的API First原则
2. **完成文件管理模块重构** - 统一文件操作业务逻辑
3. **添加API版本管理** - 支持向后兼容和渐进式升级
4. **完善监控和日志** - 提供业务级别的可观测性
5. **性能优化** - 基于清晰的架构进行针对性优化

## 📝 注意事项

1. **保持向后兼容**：在迁移过程中，确保现有功能不受影响
2. **渐进式迁移**：分阶段进行，避免大爆炸式重构
3. **完善测试**：确保新架构的功能和性能符合预期
4. **文档更新**：及时更新API文档和架构文档
5. **团队培训**：确保团队理解新架构的设计原则和使用方法

通过这次API First重构，您的项目将获得更好的可维护性、可扩展性和代码复用性。