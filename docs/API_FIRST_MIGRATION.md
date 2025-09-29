# API First 架构迁移指南

## 概述

本文档描述了从混合架构向API First + DDD架构的迁移过程，消除重复代码，建立统一的业务服务层。

## 架构对比

### 迁移前的问题
```
┌─────────────────┐    ┌─────────────────┐
│   REST Handler  │    │ Telegram Handler│
│                 │    │                 │
│ • 直接调用Aria2 │    │ • 调用App服务   │
│ • 重复业务逻辑  │    │ • 部分重复逻辑  │
│ • 配置重复加载  │    │ • 消息格式转换  │
└─────────────────┘    └─────────────────┘
         │                       │
         ▼                       ▼
┌─────────────────┐    ┌─────────────────┐
│ Aria2 Client    │    │ Application     │
│                 │    │ Services        │
└─────────────────┘    └─────────────────┘
```

### 迁移后的架构
```
┌─────────────────┐    ┌─────────────────┐
│ REST Handler V2 │    │Telegram HandlerV2│
│                 │    │                 │
│ • 纯协议转换    │    │ • 纯协议转换    │
│ • 无业务逻辑    │    │ • 无业务逻辑    │
│ • 统一错误处理  │    │ • 统一错误处理  │
└─────────────────┘    └─────────────────┘
         │                       │
         ▼                       ▼
┌─────────────────────────────────────────┐
│          Service Container              │
│                                         │
│ ┌─────────────┐ ┌─────────────────────┐ │
│ │ Download    │ │ File Service        │ │
│ │ Service     │ │                     │ │
│ └─────────────┘ └─────────────────────┘ │
│ ┌─────────────┐ ┌─────────────────────┐ │
│ │ Task        │ │ Notification        │ │
│ │ Service     │ │ Service             │ │
│ └─────────────┘ └─────────────────────┘ │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│        Infrastructure Layer            │
│                                         │
│ ┌─────────────┐ ┌─────────────────────┐ │
│ │ Aria2       │ │ Alist Client        │ │
│ │ Client      │ │                     │ │
│ └─────────────┘ └─────────────────────┘ │
└─────────────────────────────────────────┘
```

## 核心改进

### 1. 业务契约层 (Contracts)
统一定义所有业务接口和数据传输对象：

```go
// 统一的下载请求格式
type DownloadRequest struct {
    URL          string                 `json:"url"`
    Filename     string                 `json:"filename,omitempty"`
    Directory    string                 `json:"directory,omitempty"`
    Options      map[string]interface{} `json:"options,omitempty"`
    VideoOnly    bool                   `json:"video_only,omitempty"`
    AutoClassify bool                   `json:"auto_classify,omitempty"`
}

// 统一的业务服务接口
type DownloadService interface {
    CreateDownload(ctx context.Context, req DownloadRequest) (*DownloadResponse, error)
    GetDownload(ctx context.Context, id string) (*DownloadResponse, error)
    ListDownloads(ctx context.Context, req DownloadListRequest) (*DownloadListResponse, error)
    // ... 其他方法
}
```

### 2. 应用服务层 (Application Services)
实现统一的业务逻辑，消除重复：

```go
type AppDownloadService struct {
    config      *config.Config
    aria2Client *aria2.Client
    fileService contracts.FileService
}

func (s *AppDownloadService) CreateDownload(ctx context.Context, req contracts.DownloadRequest) (*contracts.DownloadResponse, error) {
    // 1. 参数验证
    if err := s.validateDownloadRequest(req); err != nil {
        return nil, err
    }

    // 2. 应用业务规则
    if err := s.applyBusinessRules(&req); err != nil {
        return nil, err
    }

    // 3. 准备下载选项
    options := s.prepareDownloadOptions(req)

    // 4. 创建Aria2下载任务
    gid, err := s.aria2Client.AddURI(req.URL, options)
    if err != nil {
        return nil, fmt.Errorf("failed to create download: %w", err)
    }

    // 5. 构建响应
    return s.buildDownloadResponse(gid, req), nil
}
```

### 3. 纯协议转换层 (Protocol Adapters)

#### REST Handler V2
```go
func (h *DownloadHandler) CreateDownload(c *gin.Context) {
    // 1. HTTP协议转换 - 解析请求
    var req contracts.DownloadRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        utils.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
        return
    }

    // 2. 调用应用服务 - 业务逻辑委托
    downloadService := h.container.GetDownloadService()
    response, err := downloadService.CreateDownload(c.Request.Context(), req)
    if err != nil {
        h.handleServiceError(c, err)
        return
    }

    // 3. HTTP协议转换 - 返回响应
    utils.Success(c, gin.H{
        "message":  "Download created successfully",
        "download": response,
    })
}
```

#### Telegram Handler V2
```go
func (dc *DownloadCommandsV2) handleURLDownload(ctx context.Context, chatID int64, url string) {
    // 1. Telegram协议转换 - 构建请求
    req := contracts.DownloadRequest{
        URL:          url,
        AutoClassify: true,
    }

    // 2. 调用应用服务 - 相同的业务逻辑
    downloadService := dc.container.GetDownloadService()
    response, err := downloadService.CreateDownload(ctx, req)
    if err != nil {
        dc.messageUtils.SendMessage(chatID, "创建下载任务失败: "+err.Error())
        return
    }

    // 3. Telegram协议转换 - 格式化消息
    message := fmt.Sprintf("<b>下载任务已创建</b>\\n\\nURL: <code>%s</code>\\nGID: <code>%s</code>",
        dc.messageUtils.EscapeHTML(url), 
        dc.messageUtils.EscapeHTML(response.ID))
    dc.messageUtils.SendMessageHTML(chatID, message)
}
```

## 迁移步骤

### Phase 1: 创建业务契约层
- [x] 定义 `contracts/download_contract.go`
- [x] 定义 `contracts/file_contract.go`
- [x] 定义 `contracts/task_contract.go`
- [x] 定义 `contracts/notification_contract.go`
- [x] 定义 `contracts/common_types.go`

### Phase 2: 实现应用服务层
- [x] 实现 `AppDownloadService` - 统一下载业务逻辑
- [x] 创建 `ServiceContainer` - 依赖注入容器
- [ ] 实现 `AppFileService` - 统一文件业务逻辑
- [ ] 实现 `AppTaskService` - 统一任务业务逻辑
- [ ] 实现 `AppNotificationService` - 统一通知业务逻辑

### Phase 3: 重构协议适配层
- [x] 创建 `handlers/download_v2.go` - 纯REST协议转换
- [x] 创建 `handlers/task_v2.go` - 纯REST协议转换
- [x] 创建 `telegram/commands/download_commands_v2.go` - 纯Telegram协议转换
- [ ] 迁移其他Telegram命令处理器

### Phase 4: 建立路由和中间件
- [x] 创建 `routes/v2_routes.go` - V2版本路由配置
- [ ] 实现依赖注入中间件
- [ ] 实现统一错误处理中间件

### Phase 5: 向下兼容性保证
- [ ] 保持V1路由继续工作
- [ ] 添加API版本协商
- [ ] 创建迁移脚本

## 使用示例

### 旧方式 (V1) - 重复代码
```go
// REST Handler
func CreateDownload(c *gin.Context) {
    cfg, err := config.LoadConfig()  // 重复配置加载
    aria2Client := aria2.NewClient(cfg.Aria2.RpcURL, cfg.Aria2.Token)  // 重复客户端创建
    // 重复的业务逻辑...
}

// Telegram Handler
func HandleDownload(chatID int64, url string) {
    cfg, err := config.LoadConfig()  // 重复配置加载
    downloadService := services.NewDownloadService()  // 不一致的服务调用
    // 重复的业务逻辑...
}
```

### 新方式 (V2) - 统一业务逻辑
```go
// 应用服务 - 统一业务逻辑
type AppDownloadService struct {
    // 单例配置和客户端
}

// REST Handler - 纯协议转换
func (h *DownloadHandler) CreateDownload(c *gin.Context) {
    req := parseHTTPRequest(c)
    response := h.container.GetDownloadService().CreateDownload(ctx, req)
    sendHTTPResponse(c, response)
}

// Telegram Handler - 纯协议转换
func (dc *DownloadCommandsV2) HandleDownload(chatID int64, url string) {
    req := parseTelegramCommand(url)
    response := dc.container.GetDownloadService().CreateDownload(ctx, req)
    sendTelegramMessage(chatID, response)
}
```

## 优势总结

### 1. 消除重复代码
- 业务逻辑只在应用服务层实现一次
- 配置和客户端单例管理
- 错误处理统一化

### 2. API First原则
- 统一的业务契约，多客户端共享
- 协议无关的业务逻辑
- 易于添加新的协议适配器

### 3. DDD架构优势
- 清晰的分层和职责划分
- 业务逻辑与技术细节分离
- 依赖方向正确（向内依赖）

### 4. 可测试性
- 业务逻辑易于单元测试
- 依赖注入便于Mock
- 协议转换层轻量化

### 5. 可维护性
- 单一职责原则
- 开闭原则（易于扩展）
- 向下兼容的迁移路径

## 兼容性保证

- V1 API继续工作
- 逐步迁移，不破坏现有功能
- 配置向下兼容
- 数据库结构不变

## 下一步计划

1. 完成所有应用服务的实现
2. 迁移所有现有Handler到V2
3. 添加V2 API的完整测试覆盖
4. 性能测试和优化
5. 文档更新和用户迁移指南