# 日志优化指南

## 📋 当前问题

### 1. 过度使用 Emoji 和中文
- ❌ `logger.Info("✅ 使用 PathStrategyService 生成路径", ...)`
- ✅ `logger.Debug("Path generated via PathStrategyService", ...)`

### 2. 日志级别使用不当
- **Info**: 业务关键事件（下载开始/完成、用户操作）
- **Debug**: 调试信息（路径计算、内部状态）
- **Warn**: 可恢复的问题（回退逻辑、配置缺失）
- **Error**: 错误情况（失败的操作）

### 3. 日志过于冗余
- 每个中间步骤都记录 → 只记录关键节点
- 使用 Debug 级别记录调试信息

## 🎯 优化原则

### 1. 移除所有 Emoji
```go
// ❌ 不推荐
logger.Info("✅ 使用 PathStrategyService 生成路径", "file", file.Name)
logger.Info("🎯 使用智能电视剧路径", "path", smartPath)
logger.Info("🔍 路径组件分析", "pathParts", pathParts)

// ✅ 推荐
logger.Debug("Path generated via PathStrategyService", "file", file.Name)
logger.Debug("Using smart TV path", "path", smartPath)
logger.Debug("Analyzing path components", "pathParts", pathParts)
```

### 2. 统一使用英文
```go
// ❌ 不推荐
logger.Info("路径分类分析（旧逻辑）", "path", file.Path)
logger.Warn("无法从目录名提取季度编号", "dirName", dirName)

// ✅ 推荐
logger.Debug("Analyzing path category (legacy)", "path", file.Path)
logger.Debug("Failed to extract season from directory", "dirName", dirName)
```

### 3. 正确使用日志级别
```go
// Info - 业务关键事件
logger.Info("Download created successfully", "id", gid, "filename", filename)
logger.Info("Download paused", "id", id)
logger.Info("File batch processing completed", "total", len(files), "success", successCount)

// Debug - 调试信息（内部流程）
logger.Debug("Extracting season number", "dirName", dirName, "seasonNum", seasonNum)
logger.Debug("Path structure extracted", "original", path, "extracted", result)
logger.Debug("Template rendered", "template", tmpl, "result", rendered)

// Warn - 可恢复的问题
logger.Warn("Failed to get global stats", "error", err)
logger.Warn("PathStrategyService failed, using fallback", "error", err)
logger.Warn("Configuration missing, using defaults", "key", configKey)

// Error - 不可恢复的错误
logger.Error("Failed to create download", "error", err, "url", req.URL)
logger.Error("File not found", "path", filePath, "error", err)
logger.Error("Database connection failed", "error", err)
```

### 4. 简化冗余日志
```go
// ❌ 不推荐 - 每一步都记录
logger.Info("🔍 开始分析路径", "path", path)
logger.Info("🔍 提取路径片段", "keyword", keyword)
logger.Info("🔍 过滤分类关键词", "original", path)
logger.Info("🔍 清理节目名", "name", name)
logger.Info("✅ 路径分析完成", "result", result)

// ✅ 推荐 - 只记录关键节点
logger.Debug("Analyzing path", "path", path, "result", result)
// 如果需要详细调试，使用一条日志包含所有信息
logger.Debug("Path analysis details",
    "path", path,
    "keyword", keyword,
    "filtered", filtered,
    "cleaned", cleaned,
    "result", result)
```

## 📝 具体优化建议

### app_file_utils.go (39条日志)
**优化方案**: 减少到 8-10 条关键日志

```go
// 保留 - Info 级别
- Download path generated (最终结果)
- Path category detected (分类结果)

// 改为 Debug 级别
- 所有中间步骤的日志
- 路径解析细节
- 季度/集数提取过程
```

### app_file_service.go (20条日志)
**优化方案**: 减少到 10-12 条

```go
// 保留 - Info 级别
- Service initialized
- File processing started/completed

// 改为 Debug 级别
- Path strategy initialization
- Template rendering details
```

### path_strategy_service.go (11条日志)
**优化方案**: 减少到 5-6 条

```go
// 保留
- Strategy selection
- Generation success/failure

// 改为 Debug
- Template mode check
- Variable extraction details
```

## 🔧 实施步骤

### 第一阶段：批量替换
```bash
# 1. 移除所有 emoji
find internal/application/services -name "*.go" -exec sed -i 's/logger\.Info("\([^"]*\)[✅❌⚠️🎯📁🔍🚀📋🧹]\+/logger.Debug("\1/g' {} +

# 2. Info 改为 Debug（选择性）
# 手动审查每个 logger.Info，判断是否应该改为 Debug
```

### 第二阶段：逐文件优化
优先处理日志最多的文件：
1. app_file_utils.go (39条)
2. app_file_service.go (20条)
3. directory_manager.go (16条)
4. path_strategy_service.go (11条)
5. path_mapping_engine.go (10条)

### 第三阶段：统一规范
- 所有新增日志遵循本指南
- Code Review 时检查日志规范
- 添加 CI 检查（可选）

## 📊 日志级别分布目标

```
当前分布:
Info:  ~120 条
Warn:  ~20 条
Error: ~30 条
Debug: ~2 条

优化后目标:
Info:  ~30-40 条  (业务关键事件)
Warn:  ~15-20 条  (可恢复问题)
Error: ~25-30 条  (错误情况)
Debug: ~80-100 条 (调试信息)
```

## ✅ 检查清单

在提交代码前，检查：
- [ ] 没有使用 emoji
- [ ] 使用英文描述
- [ ] Info 级别仅用于业务关键事件
- [ ] 调试信息使用 Debug 级别
- [ ] 错误信息包含足够的上下文（error、相关参数）
- [ ] 避免循环中大量日志输出
- [ ] 日志消息简洁明了

## 🚀 长期改进

考虑升级日志库到结构化日志：
- 使用 Go 1.21+ 的 `slog` 标准库
- 或使用 `zap`/`zerolog` 高性能日志库
- 支持 JSON 格式输出
- 支持日志级别动态调整
- 更好的性能和结构化字段支持
