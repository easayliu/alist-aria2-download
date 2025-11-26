# 日志使用规范

本文档定义了项目中日志使用的最佳实践和规范。

## 目录

- [日志级别](#日志级别)
- [安全日志](#安全日志)
- [日志格式](#日志格式)
- [性能考虑](#性能考虑)
- [示例](#示例)

## 日志级别

### Debug
**用途**: 详细的调试信息,仅在开发环境或排查问题时使用

**使用场景**:
- 函数参数和返回值
- 循环中的详细信息
- 算法执行步骤
- 性能分析数据

**示例**:
```go
logger.Debug("URL replaced",
    "original", originalURL,
    "internal", internalURL)
```

### Info
**用途**: 重要的业务流程和系统状态信息

**使用场景**:
- 系统启动/停止
- 批量操作的汇总统计
- 业务流程的关键节点
- 配置加载成功
- 定时任务执行

**示例**:
```go
logger.Info("Batch rename completed",
    "total", 100,
    "success", 95,
    "failed", 5,
    "success_rate", "95.0%",
    "duration", "2m30s")
```

### Warn
**用途**: 可恢复的错误和异常情况

**使用场景**:
- 可恢复的错误
- 资源接近限制
- 配置缺失使用默认值
- 重试操作
- 降级策略触发

**示例**:
```go
logger.Warn("Failed to get file info, skipping",
    "path", fullPath,
    "error", err,
    "file_name", file.Name)
```

### Error
**用途**: 需要立即关注的错误

**使用场景**:
- 业务操作失败
- 数据一致性问题
- 外部依赖故障
- 无法恢复的错误
- 系统异常

**示例**:
```go
logger.Error("TMDB API Request failed",
    "endpoint", endpoint,
    "error", err)
```

## 安全日志

### 敏感信息处理

项目提供了自动脱敏的Safe方法,用于处理可能包含敏感信息的日志。

**敏感字段**包括:
- token
- password / passwd / pwd
- api_key / apikey
- secret
- authorization / auth

### 使用Safe方法

当日志参数中可能包含敏感信息时,使用`*Safe`方法:

```go
// ❌ 错误 - 可能泄露token
logger.Warn("Path token not found", "token", encoded)

// ✅ 正确 - 自动脱敏
logger.WarnSafe("Path token not found", "token", encoded)
```

### 脱敏效果

```go
// 输入: token="Bearer_1234567890abcdefghij"
// 输出: token="Bear*******************ghij"

// 短token(< 8字符)完全隐藏
// 输入: token="abc123"
// 输出: token="***"
```

### 何时使用Safe方法

**必须使用**:
- 输出任何token、password、api_key等字段时
- 配置加载相关的日志
- 认证授权相关的日志

**可以不用**:
- 纯业务数据(文件名、路径、大小等)
- 不包含敏感信息的日志

## 日志格式

### 结构化日志

始终使用键值对格式:

```go
// ✅ 正确 - 结构化
logger.Info("File processed",
    "filename", file.Name,
    "size", file.Size,
    "duration", elapsed.String())

// ❌ 错误 - 字符串拼接
logger.Info(fmt.Sprintf("File %s processed, size=%d", file.Name, file.Size))
```

### 键名规范

- 使用snake_case: `file_name`, `success_rate`, `total_count`
- 保持一致性: 相同含义使用相同的键名
- 避免缩写: `filename` 而非 `fn`

### 常用键名

| 场景 | 推荐键名 |
|------|---------|
| 文件操作 | `filename`, `path`, `size`, `old_path`, `new_path` |
| 批量操作 | `total`, `success`, `failed`, `progress_pct` |
| 时间相关 | `duration`, `elapsed`, `avg_per_file` |
| 错误信息 | `error`, `reason`, `code` |
| 标识符 | `id`, `task_id`, `user_id` |

## 性能考虑

### 循环中的日志

#### ❌ 不好的做法
```go
for _, file := range files {
    logger.Debug("Processing file", "name", file.Name)
    // 1000个文件 = 1000条日志
}
```

#### ✅ 推荐做法

**方式1: Debug详细 + Info汇总**
```go
stats := struct{ total, success, failed int }{}

for _, file := range files {
    stats.total++
    err := processFile(file)
    if err != nil {
        stats.failed++
        logger.Debug("File processing failed",
            "name", file.Name,
            "error", err)
    } else {
        stats.success++
    }
}

logger.Info("File processing completed",
    "total", stats.total,
    "success", stats.success,
    "failed", stats.failed)
```

### 大对象日志

避免在日志中输出大对象:

```go
// ❌ 不好
logger.Debug("Config loaded", "config", cfg) // cfg可能很大

// ✅ 推荐
logger.Debug("Config loaded",
    "items_count", len(cfg.Items),
    "enabled", cfg.Enabled)
```

## 示例

### 完整的批量操作日志

```go
func (s *Service) BatchProcess(items []Item) error {
    startTime := time.Now()
    logger.Info("Batch processing started", "total", len(items))

    var successCount, failedCount int

    for _, item := range items {
        if err := s.process(item); err != nil {
            failedCount++
            logger.Debug("Item processing failed",
                "id", item.ID,
                "error", err)
        } else {
            successCount++
        }
    }

    duration := time.Since(startTime)
    logger.Info("Batch processing completed",
        "total", len(items),
        "success", successCount,
        "failed", failedCount,
        "duration", duration.String(),
        "avg_per_item", (duration / time.Duration(len(items))).String())

    return nil
}
```

### 带进度报告的长时间操作

```go
func (s *Service) LongRunningTask(tasks []Task) {
    startTime := time.Now()
    var processed int32

    // 进度报告
    ticker := time.NewTicker(10 * time.Second)
    go func() {
        for range ticker.C {
            current := atomic.LoadInt32(&processed)
            progress := float64(current) / float64(len(tasks)) * 100
            logger.Info("Task progress",
                "processed", current,
                "total", len(tasks),
                "progress_pct", fmt.Sprintf("%.1f%%", progress),
                "elapsed", time.Since(startTime).String())
        }
    }()

    // 执行任务...

    ticker.Stop()
}
```

### 敏感信息处理

```go
func (s *Service) Login(username, password string) error {
    // ❌ 危险
    logger.Debug("Login attempt",
        "username", username,
        "password", password) // 明文密码!

    // ✅ 安全
    logger.DebugSafe("Login attempt",
        "username", username,
        "password", password) // 自动脱敏

    // 或者手动脱敏
    logger.Debug("Login attempt",
        "username", username,
        "password_masked", logger.MaskToken(password))
}
```

## 检查清单

在提交代码前,检查以下项目:

- [ ] 所有日志使用了正确的级别
- [ ] 循环中的日志已优化(使用汇总或采样)
- [ ] 敏感信息使用了Safe方法或手动脱敏
- [ ] 使用了结构化的键值对格式
- [ ] 错误日志包含足够的上下文信息
- [ ] 批量操作有开始、进度和完成日志
- [ ] 没有输出大对象到日志中

## 工具函数

### 脱敏函数

```go
// MaskToken - 脱敏token(保留前4后4)
masked := logger.MaskToken(token)

// SanitizeValue - 根据键名智能脱敏
value := logger.SanitizeValue("api_key", rawValue)
```

### Safe日志方法

```go
logger.DebugSafe(msg, args...)
logger.InfoSafe(msg, args...)
logger.WarnSafe(msg, args...)
logger.ErrorSafe(msg, args...)
```

## 参考

- [Structured Logging](https://www.honeycomb.io/blog/structured-logging-and-your-team)
- [Log Levels](https://sematext.com/blog/logging-levels/)
- [Go log/slog Package](https://pkg.go.dev/log/slog)
