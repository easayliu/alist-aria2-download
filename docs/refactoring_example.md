# Handler重构示例

## Before - 每个Handler重复创建客户端 (❌ 不推荐)

```go
func ManualDownloadFiles(c *gin.Context) {
    var req ManualDownloadRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        utils.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request parameters: "+err.Error())
        return
    }

    // ❌ 重复代码: 加载配置
    cfg, err := config.LoadConfig()
    if err != nil {
        utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to load config")
        return
    }

    // ❌ 重复代码: 创建Alist客户端
    alistClient := alist.NewClient(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password)

    // ❌ 重复代码: 创建文件服务
    fileService := services.NewFileService(alistClient)

    // ... 业务逻辑
}
```

## After - 使用ServiceContainer (✅ 推荐)

```go
func ManualDownloadFiles(c *gin.Context) {
    var req ManualDownloadRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        utils.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request parameters: "+err.Error())
        return
    }

    // ✅ 统一方式: 从context获取container
    container := handlers.GetContainer(c)
    cfg := container.GetConfig()

    // ✅ 统一方式: 从container获取服务
    fileService := container.GetFileService()

    // ... 业务逻辑 (相同)
}
```

## 使用Container的优势

### 1. 代码减少
- Before: 每个handler ~15行样板代码
- After: 每个handler ~2行获取依赖
- **减少87% 样板代码**

### 2. 一致性
- 所有handler使用相同方式获取依赖
- 配置和客户端由container统一管理
- 避免不一致的初始化方式

### 3. 可测试性
- 可以轻松mock ServiceContainer
- 不需要mock config.LoadConfig()
- 不需要mock各种Client构造函数

### 4. 性能优化
- Config只加载一次(在应用启动时)
- Client实例复用,减少创建开销
- 减少重复的IO操作

### 5. 错误处理
- 统一的初始化失败处理
- 应用启动时就发现配置问题
- 不会在运行时才发现配置错误

## Setup - 在main.go中设置

```go
func main() {
    // 加载配置
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Fatal("Failed to load config:", err)
    }

    // 创建服务容器
    container, err := services.NewServiceContainer(cfg)
    if err != nil {
        log.Fatal("Failed to create service container:", err)
    }

    // 创建路由
    router := gin.Default()

    // ✅ 添加Container中间件
    router.Use(middleware.ContainerMiddleware(container))

    // 设置路由
    router.POST("/api/v1/files/manual-download", handlers.ManualDownloadFiles)

    // 启动服务器
    router.Run(":8080")
}
```

## 重构步骤

### Step 1: 识别重复模式
在当前项目中发现的重复模式:
- `config.LoadConfig()` - 55次
- `alist.NewClient()` - 18次
- `aria2.NewClient()` - 12次
- `services.NewFileService()` - 15次

### Step 2: 使用Container替换

Replace这样的代码:
```go
cfg, err := config.LoadConfig()
if err != nil {
    return err
}
alistClient := alist.NewClient(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password)
fileService := services.NewFileService(alistClient)
```

变成:
```go
container := handlers.GetContainer(c)
fileService := container.GetFileService()
```

### Step 3: 更新所有handlers
按优先级重构:
1. file_handler.go (15次重复) - 最高优先级
2. file_api.go (4次重复)
3. alist.go (6次重复)
4. download.go (12次重复)

### Step 4: 删除不需要的import
重构后可以删除的import:
```go
// ❌ 不再需要
import (
    "github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
    "github.com/easayliu/alist-aria2-download/internal/infrastructure/aria2"
    "github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
)
```

## 预期成果

| 指标 | Before | After | 改进 |
|-----|--------|-------|------|
| 总代码行数 | 8202行 | ~6500行 | -21% |
| 配置加载次数 | 每次请求 | 启动时1次 | -99% |
| Client创建次数 | 每次请求 | 启动时1次 | -99% |
| 样板代码 | ~825行 | ~110行 | -87% |
| Handler平均行数 | 85行 | 70行 | -18% |

## 兼容性

✅ 完全向后兼容
- 不影响现有API接口
- 不改变业务逻辑
- 只改变内部实现方式

## 下一步

1. ✅ 创建Container中间件
2. ✅ 创建BaseHandler辅助类
3. 🔄 重构file_handler.go
4. ⏳ 重构file_api.go
5. ⏳ 重构alist.go
6. ⏳ 运行测试验证
