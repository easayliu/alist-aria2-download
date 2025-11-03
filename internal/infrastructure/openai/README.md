# OpenAI 基础客户端

这是一个轻量级的 OpenAI API 客户端实现,支持 Chat Completion API (非流式和流式)。

## 功能特性

- ✅ 支持自定义 base_url (兼容第三方 OpenAI API,如 OneAPI)
- ✅ 支持 API Key 认证 (支持环境变量)
- ✅ 集成速率限制 (QPS 控制)
- ✅ 完善的错误处理
- ✅ 超时控制
- ✅ 支持非流式和流式响应
- ✅ 日志集成
- ✅ 线程安全

## 快速开始

### 1. 基本配置

```go
import "github.com/easayliu/alist-aria2-download/internal/infrastructure/openai"

// 方式1: 直接创建配置
cfg := &openai.Config{
    APIKey:      "sk-xxx",  // 或从环境变量 OPENAI_API_KEY 读取
    BaseURL:     "https://api.openai.com/v1",
    Model:       "gpt-3.5-turbo",
    Temperature: 0.3,
    MaxTokens:   1000,
    Timeout:     30 * time.Second,
    QPS:         10,  // 每秒最多10个请求
}

// 方式2: 从应用配置创建
appCfg, _ := config.LoadConfig()
cfg := openai.NewConfigFromAppConfig(&appCfg.LLM.OpenAI)
```

### 2. 创建客户端

```go
client, err := openai.NewClient(cfg)
if err != nil {
    log.Fatal(err)
}
```

### 3. 非流式请求

```go
req := &openai.ChatRequest{
    Model: "gpt-3.5-turbo",
    Messages: []openai.ChatMessage{
        {Role: "user", Content: "你好，请介绍一下自己"},
    },
    Temperature: 0.7,
}

ctx := context.Background()
resp, err := client.ChatCompletion(ctx, req)
if err != nil {
    log.Fatal(err)
}

fmt.Println(resp.Choices[0].Message.Content)
```

### 4. 流式请求

```go
req := &openai.ChatRequest{
    Model: "gpt-3.5-turbo",
    Messages: []openai.ChatMessage{
        {Role: "user", Content: "讲一个故事"},
    },
}

handler, err := client.ChatCompletionStream(ctx, req)
if err != nil {
    log.Fatal(err)
}

chunkChan, errChan := handler.ReadStream(ctx)
for chunk := range chunkChan {
    content := openai.ExtractContent(chunk)
    fmt.Print(content)  // 实时输出
}

if err := <-errChan; err != nil {
    log.Fatal(err)
}
```

### 5. 使用 ChatClient 简化 API

```go
chatClient := openai.NewChatClient(client)

// 简单非流式调用
reply, err := chatClient.SimpleComplete(ctx, "你好")

// 简单流式调用
textChan, errChan := chatClient.SimpleCompleteStream(ctx, "讲个故事")
for text := range textChan {
    fmt.Print(text)
}
```

## 配置说明

### 环境变量支持

API Key 优先从环境变量 `OPENAI_API_KEY` 读取,如果环境变量不存在才使用配置文件中的值。

```bash
export OPENAI_API_KEY="sk-xxx"
```

### 第三方 API 支持

通过修改 `base_url` 可以使用第三方 OpenAI 兼容 API:

```yaml
llm:
  openai:
    base_url: "http://localhost:3000/v1"  # 例如 OneAPI
    api_key: "your-key"
```

### 速率限制

通过 `qps` 参数控制每秒请求数,防止超过 API 配额:

```go
cfg.QPS = 10  // 每秒最多10个请求,0表示不限制
```

## 错误处理

客户端定义了标准错误类型:

```go
- ErrMissingAPIKey: API密钥缺失
- ErrInvalidConfig: 配置无效
- ErrRequestFailed: 请求失败
- ErrRateLimitExceeded: 速率限制
- ErrEmptyResponse: 空响应
- ErrContextCanceled: 请求取消
```

## 日志

客户端集成了项目日志系统,会输出:
- 客户端创建信息 (INFO级别)
- 请求和响应详情 (DEBUG级别)
- 错误信息 (ERROR级别)

## 线程安全

所有客户端方法都是线程安全的,可以在多个goroutine中共享同一个客户端实例。

## 架构说明

```
openai/
├── types.go          # 请求/响应类型定义
├── config.go         # 配置管理
├── errors.go         # 错误定义
├── client.go         # 基础客户端 (HTTP请求封装)
├── chat_client.go    # Chat专用客户端 (高级API)
├── stream_handler.go # 流式响应处理
└── example_test.go   # 使用示例
```

## 依赖

- `github.com/sashabaranov/go-openai`: OpenAI Go SDK (仅用于参考,未直接使用)
- `internal/infrastructure/ratelimit`: 项目内置速率限制器
- `pkg/logger`: 项目日志系统
