# LLM重命名架构优化

## 变更概述

将混合策略(LLM+TMDB fallback)简化为**纯LLM策略**,提高一致性和可维护性。

## 旧架构问题

### 1. 复杂的混合逻辑
```
LLM推断 → 部分失败 → 过滤特殊内容 → TMDB重试 → 合并结果
```

**问题**:
- 逻辑复杂,难以调试
- 特殊内容被跳过多次
- TMDB无法处理综艺等特殊内容
- 结果来源不一致(部分LLM,部分TMDB)

### 2. 索引匹配Bug
```go
for i, llmResult := range llmResults {
    originalPath = paths[i]  // ❌ 假设顺序一致
}
```

当LLM返回结果数量少于输入时,索引错位导致全部失败。

### 3. 缺失结果未处理
LLM返回46个结果但输入60个文件时,后14个文件被忽略。

## 新架构

### 核心原则

**完全信任LLM,移除TMDB fallback**

```
┌─────────────────────────────────────┐
│  GetBatchRenameSuggestionsWithLLM   │
└─────────────┬───────────────────────┘
              │
         ┌────┴────┐
         │ LLM启用? │
         └────┬────┘
              │
      ┌───────┴───────┐
      │               │
   是 │               │ 否
      ▼               ▼
  ┌─────────┐    ┌─────────┐
  │ 纯LLM   │    │ 纯TMDB  │
  │ 推断    │    │ 推断    │
  └────┬────┘    └─────────┘
       │
       ▼
  ┌──────────────┐
  │ 文件名匹配   │
  │ (非索引匹配) │
  └──────┬───────┘
         │
         ▼
  ┌──────────────┐
  │ 处理结果     │
  │ - 成功:添加  │
  │ - 失败:跳过  │
  └──────────────┘
```

### 关键改进

#### 1. **移除TMDB Fallback**
```go
// 旧代码
if llmResult.Error != "" {
    failedPaths = append(failedPaths, path)
    // 后续用TMDB重试
}

// 新代码
if llmResult.Error != "" {
    logger.Info("LLM无法处理此文件", "path", path)
    skippedCount++
    // 不添加到结果,不重试
    continue
}
```

**优点**:
- ✅ 逻辑简单清晰
- ✅ 结果来源一致(全部LLM)
- ✅ 避免无效的TMDB调用
- ✅ 用户明确知道哪些文件无法处理

#### 2. **文件名匹配替代索引匹配**
```go
// 创建文件名到路径的映射
pathMap := make(map[string]string)
for _, path := range paths {
    pathMap[filepath.Base(path)] = path
}

// 使用OriginalName查找
for _, llmResult := range llmResults {
    originalPath, found := pathMap[llmResult.OriginalName]
    if !found {
        logger.Warn("Cannot find path for LLM result")
        continue
    }
    // 正确处理...
}
```

**优点**:
- ✅ 不依赖顺序
- ✅ 明确匹配关系
- ✅ 处理结果数量不一致的情况

#### 3. **补充缺失结果**
在`batch_llm_suggester.go`中:
```go
// 检查是否有未处理的文件
processedFiles := make(map[string]bool)

for _, result := range output.Results {
    processedFiles[result.OriginalName] = true
}

// 为缺失的文件生成Error
for _, file := range batch {
    if !processedFiles[file.OriginalName] {
        results = append(results, BatchFileNameSuggestion{
            OriginalName: file.OriginalName,
            Error:        "LLM未返回此文件的结果",
        })
    }
}
```

**优点**:
- ✅ 确保所有文件都有结果(成功或Error)
- ✅ 易于诊断问题
- ✅ 日志清晰

## 用户体验改进

### Telegram消息优化

**旧消息**:
```
❌ 2025.08.09_首映篇：...
   未找到匹配的电影/剧集
```

**新消息**:
```
⚠️ 2025.08.09_首映篇：...
   特殊内容暂不支持重命名
```

### 日志优化

**旧日志**:
```
[WARN] No TMDB suggestions found filePath=...
[INFO] 部分文件LLM失败,使用TMDB重试 failedCount=14
```

**新日志**:
```
[INFO] LLM无法处理特殊内容 filePath=...
[INFO] 批量LLM推断完成 successCount=46 skippedCount=14
```

## 特殊内容处理

### 识别的特殊内容
- 加更版、首映篇、特别企划、收官篇、先导片
- 特辑、番外、精华版、幕后特辑、制作特辑
- 演唱会、见面会、发布会、粉丝见面会
- 花絮、删减片段、未播片段、片场花絮
- SP特别篇、OVA、剧场版、总集篇
- 导演剪辑版、未删减版、加长版

### 处理策略
1. LLM尝试识别为E00特殊版本
2. 如果LLM无法处理,跳过(不fallback TMDB)
3. 用户看到"特殊内容暂不支持重命名"提示

## 配置要求

LLM必须正确配置才能使用:
```yaml
llm:
  enabled: true
  provider: "openai"
  openai:
    api_key: "your-api-key"
    base_url: "https://api.qnaigc.com/v1"
    model: "doubao-1.5-pro-32k"
```

## 代码变更清单

### 修改的文件
1. `file_rename.go` - 移除TMDB fallback逻辑
2. `batch_llm_suggester.go` - 补充缺失结果
3. `telegram_batch_rename_handler.go` - 优化用户消息(预览和确认两个函数)

### 删除的代码
- ❌ TMDB fallback逻辑
- ❌ 特殊内容过滤逻辑(用于fallback)
- ❌ 索引匹配逻辑

### 新增的代码
- ✅ 文件名匹配逻辑
- ✅ 缺失结果检测
- ✅ 特殊内容友好提示

## 测试建议

### 测试场景
1. **正常剧集**: 60个文件全部成功
2. **特殊内容**: 14个特殊内容跳过,46个成功
3. **混合内容**: 部分成功,部分跳过
4. **LLM未启用**: 自动使用TMDB
5. **LLM API失败**: 返回错误,不fallback

### 预期结果
```
[INFO] 使用LLM批量推断模式 fileCount=60
[INFO] LLM batch response received requestedFiles=60 returnedResults=60
[INFO] 批量LLM推断完成 successCount=46 skippedCount=14
[INFO] LLM无法处理特殊内容 (14次)
```

## 未来优化方向

### 1. 改进LLM对特殊内容的处理
- 更新prompt,明确说明如何处理特殊内容
- 使用episode=0来标记特殊版本
- 生成合适的文件名(如"节目名 - S07E00 - 首映篇.mp4")

### 2. 单文件重命名
考虑也使用LLM:
```go
func (s *AppFileService) GetRenameSuggestions(ctx context.Context, path string) {
    if s.llmService.IsEnabled() {
        // 使用LLM推断单个文件
    } else {
        // 使用TMDB
    }
}
```

### 3. 用户选择策略
在配置中允许用户选择:
```yaml
llm:
  fallback_to_tmdb: false  # 默认false,不fallback
```

## 总结

**核心理念**: 完全信任LLM,简化架构,提高可维护性

**主要收益**:
- ✅ 代码更简洁(-50行)
- ✅ 逻辑更清晰(单一路径)
- ✅ 性能更好(减少TMDB调用)
- ✅ 用户体验更好(明确提示)
- ✅ 易于调试(日志清晰)

**兼容性**: LLM未启用时自动使用TMDB,完全向后兼容
