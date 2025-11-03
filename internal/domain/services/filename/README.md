# 文件命名服务（Filename Services）

## 概述

本模块提供基于LLM和TMDB的智能文件重命名功能，支持多种策略的混合推断。

## 目录结构

```
filename/
├── llm_suggester.go      # LLM文件名推断器
├── hybrid_suggester.go   # 混合推断器（TMDB + LLM）
└── README.md            # 本文档
```

## 核心组件

### 1. LLMSuggester - LLM文件名推断器

使用大语言模型推断媒体文件的标题、年份、季集数等信息。

**主要功能**：
- 智能解析复杂文件名（支持中英文混合）
- 自动识别媒体类型（电影/剧集）
- 提取季度和集数信息
- 返回结构化的JSON输出

**使用示例**：

```go
import (
    "context"
    "github.com/easayliu/alist-aria2-download/internal/domain/services/filename"
)

// 创建LLM推断器
llmSuggester := filename.NewLLMSuggester(llmService)

// 推断文件名
req := filename.FileNameRequest{
    OriginalName: "电视剧名.第一季.第01集.1080p.mkv",
    FilePath:     "/data/tvs/电视剧名/Season 1/电视剧名.第一季.第01集.1080p.mkv",
    Hint:         "这是一部美剧",
}

suggestion, err := llmSuggester.SuggestFileName(context.Background(), req)
if err != nil {
    log.Fatal(err)
}

// 输出建议
fmt.Printf("媒体类型: %s\n", suggestion.MediaType)
fmt.Printf("标题: %s (%s)\n", suggestion.Title, suggestion.TitleCN)
fmt.Printf("年份: %d\n", suggestion.Year)
fmt.Printf("季集: S%02dE%02d\n", *suggestion.Season, *suggestion.Episode)
fmt.Printf("置信度: %.2f\n", suggestion.Confidence)
```

**流式推断**：

```go
// 流式推断（实时反馈）
callback := func(partialText string) error {
    fmt.Printf("正在推断: %s\n", partialText)
    return nil
}

suggestion, err := llmSuggester.SuggestFileNameStream(ctx, req, callback)
```

### 2. HybridSuggester - 混合推断器

结合TMDB数据库和LLM能力，提供更准确的文件名推断。

**支持策略**：

| 策略 | 说明 | 适用场景 |
|------|------|----------|
| `TMDBFirst` | TMDB优先，失败时使用LLM | 大部分英文影视内容 |
| `LLMFirst` | LLM优先，失败时使用TMDB | 中文内容或复杂命名 |
| `TMDBOnly` | 仅使用TMDB | 确信存在于TMDB的内容 |
| `LLMOnly` | 仅使用LLM | TMDB未收录的内容 |
| `Compare` | 同时使用并比较结果 | 需要人工选择最佳结果 |

**使用示例**：

```go
// 创建混合推断器（TMDB优先）
hybridSuggester := filename.NewHybridSuggester(
    tmdbSuggester,
    llmSuggester,
    filename.TMDBFirst,
)

// 单个文件推断
req := filename.FileNameRequest{
    OriginalName: "The.Mandalorian.S01E01.mkv",
}

suggestion, err := hybridSuggester.SuggestFileName(ctx, req)

// 比较模式（返回多个结果）
suggestions, err := hybridSuggester.SuggestFileNameWithCompare(ctx, req)
for i, s := range suggestions {
    fmt.Printf("选项%d [%s]: %s (置信度: %.2f)\n",
        i+1, s.Source, s.ToEmbyFormat(".mkv"), s.Confidence)
}
```

## 数据结构

### FileNameRequest - 推断请求

```go
type FileNameRequest struct {
    OriginalName string // 原始文件名（必填）
    FilePath     string // 完整文件路径（可选，提供更多上下文）
    Hint         string // 用户提示（可选，如"这是HBO的剧集"）
}
```

### FileNameSuggestion - 推断结果

```go
type FileNameSuggestion struct {
    MediaType   string  // "movie" 或 "tv"
    Title       string  // 英文标题
    TitleCN     string  // 中文标题（可选）
    Year        int     // 年份
    Season      *int    // 季度（仅剧集，可为nil）
    Episode     *int    // 集数（仅剧集，可为nil）
    Confidence  float32 // 置信度 (0.0-1.0)
    RawResponse string  // LLM原始响应（调试用）
    Source      string  // 数据来源："tmdb", "llm", "hybrid"
}
```

## 输出格式

### Emby/Plex命名格式

```go
// 电影格式
suggestion.ToEmbyFormat(".mkv")
// 输出: "The Matrix (1999).mkv"

// 剧集格式
suggestion.ToEmbyFormat(".mkv")
// 输出: "Game of Thrones - S01E01.mkv"
```

## 与现有系统集成

### 转换为TMDB格式

```go
// 将FileNameSuggestion转换为TMDB的SuggestedName格式
tmdbFormat := suggestion.ToTMDBSuggestedName("/path/to/file.mkv")

// 可以直接用于现有的重命名流程
err := fileService.RenameAndMoveFile(ctx, oldPath, tmdbFormat.NewPath)
```

### 从TMDB格式转换

```go
// TMDB结果自动转换为统一格式
tmdbSuggestions, _ := tmdbSuggester.SearchAndSuggest(ctx, filePath)
suggestion := convertTMDBToFileNameSuggestion(&tmdbSuggestions[0])
```

## 错误处理

```go
suggestion, err := suggester.SuggestFileName(ctx, req)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "置信度过低"):
        // 置信度不足，需要人工确认
        log.Warn("建议置信度较低，建议人工审核")
    case strings.Contains(err.Error(), "未找到"):
        // TMDB未找到，可尝试LLM
        log.Info("TMDB未找到，切换到LLM模式")
    case strings.Contains(err.Error(), "解析失败"):
        // LLM输出格式错误
        log.Error("LLM输出解析失败，请检查prompt")
    }
}
```

## 性能优化建议

### 1. 批量处理

```go
// 避免逐个处理，使用批量API
files := []string{"file1.mkv", "file2.mkv", "file3.mkv"}

for _, file := range files {
    go func(f string) {
        suggestion, _ := suggester.SuggestFileName(ctx, filename.FileNameRequest{
            OriginalName: f,
        })
        // 处理结果
    }(file)
}
```

### 2. 缓存策略

对于相同的文件名，建议缓存推断结果：

```go
type CachedSuggester struct {
    suggester *filename.HybridSuggester
    cache     map[string]*filename.FileNameSuggestion
}

func (c *CachedSuggester) SuggestFileName(ctx context.Context, req filename.FileNameRequest) (*filename.FileNameSuggestion, error) {
    if cached, ok := c.cache[req.OriginalName]; ok {
        return cached, nil
    }

    suggestion, err := c.suggester.SuggestFileName(ctx, req)
    if err == nil {
        c.cache[req.OriginalName] = suggestion
    }

    return suggestion, err
}
```

### 3. 超时控制

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

suggestion, err := suggester.SuggestFileName(ctx, req)
```

## 测试

### 单元测试示例

```go
func TestLLMSuggester(t *testing.T) {
    // 创建mock LLM服务
    mockLLM := &MockLLMService{
        response: `{
            "media_type": "tv",
            "title": "TV Show Title",
            "title_cn": "电视剧名称",
            "year": 2020,
            "season": 1,
            "episode": 1,
            "confidence": 0.95
        }`,
    }

    suggester := filename.NewLLMSuggester(mockLLM)

    req := filename.FileNameRequest{
        OriginalName: "电视剧名称.S01E01.mkv",
    }

    suggestion, err := suggester.SuggestFileName(context.Background(), req)

    assert.NoError(t, err)
    assert.Equal(t, "tv", suggestion.MediaType)
    assert.Equal(t, "TV Show Title", suggestion.Title)
    assert.Equal(t, 1, *suggestion.Season)
}
```

## 最佳实践

### 1. 策略选择

- **英文影视内容**：使用 `TMDBFirst`（TMDB数据更权威）
- **中文影视内容**：使用 `LLMFirst`（LLM对中文理解更好）
- **综艺/纪录片**：使用 `LLMOnly`（TMDB可能未收录）
- **不确定时**：使用 `Compare`（让用户选择）

### 2. 置信度阈值

```go
if suggestion.Confidence < 0.7 {
    // 低置信度，建议人工审核
    log.Warn("置信度较低，建议人工确认")
} else if suggestion.Confidence >= 0.9 {
    // 高置信度，可自动执行
    autoRename(suggestion)
}
```

### 3. 用户提示

提供有用的hint可以提高准确性：

```go
req := filename.FileNameRequest{
    OriginalName: "第三季第五集.mkv",
    Hint:         "这是一部美剧",
}
```

## 故障排查

### 问题1：LLM返回格式错误

**症状**：`JSON解析失败` 错误

**解决方案**：
1. 检查LLM的输出格式是否符合JSON schema
2. 查看 `RawResponse` 字段，确认实际输出
3. 调整prompt模板

### 问题2：置信度总是很低

**症状**：所有结果的 `Confidence < 0.5`

**解决方案**：
1. 检查输入的文件名质量
2. 尝试提供 `FilePath` 和 `Hint`
3. 切换到更强大的LLM模型

### 问题3：TMDB和LLM结果差异大

**症状**：比较模式下两个结果完全不同

**解决方案**：
1. 检查文件名是否有歧义
2. 查看TMDB的搜索结果数量
3. 增加用户提示信息

## 与Agent-C的集成

本模块依赖Agent-C提供的LLM服务接口：

```go
// contracts/llm_contract.go
type LLMService interface {
    GenerateStructured(ctx context.Context, prompt string, schema interface{}) (string, error)
    GenerateTextStream(ctx context.Context, prompt string) (<-chan string, <-chan error)
}
```

确保在使用前已初始化LLM服务：

```go
// 在服务容器中初始化
fileService.SetLLMService(llmService)
```

## 版本历史

- **v1.0.0** (2025-10-30): 初始版本，支持LLM和混合推断
- 依赖Agent-C的LLM服务接口

## 作者

Agent-D (Claude Code)
